package main

import (
	"flag"

	"bufio"
	"encoding/json"
	l4g "github.com/alecthomas/log4go"
	"github.com/go-stomp/stomp"
	//"github.com/go-stomp/stomp/server/utils"
	"os"
	"strconv"
	"strings"
	"time"
)

var log l4g.Logger = l4g.NewLogger()

const (
	defaultPort = ":61614"
	clientID    = "clientID"
)

var (
	testFile = "test.csv"
	LOGFILE  = "client.log"
)

var (
	serverAddr  = flag.String("server", "localhost:61614", "STOMP server endpoint")
	destination = flag.String("topic", "/queue/nominatimRequest", "Destination topic")
	queueFormat = flag.String("queue", "/queue/", "Queue format")
	stop        = make(chan bool)
)

// these are the default options that work with Rabbi
var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login("guest", "guest"),
	stomp.ConnOpt.Host("/"),
}

func init() {
	log.AddFilter("stdout", l4g.INFO, l4g.NewConsoleLogWriter())
	log.AddFilter("file", l4g.DEBUG, l4g.NewFileLogWriter(LOGFILE, false))
	//
}

func main() {

	// logger configuration
	defer log.Close()

	flag.Parsed()
	flag.Parse()

	subscribed := make(chan bool)

	log.Info("main")

	go recvMessages(subscribed)
	// wait until we know the receiver has subscribed
	<-subscribed

	go sendMessages()

	<-stop
	<-stop
}

func sendMessages() {
	defer func() {
		stop <- true
	}()

	conn, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		log.Error("cannot connect to server", err.Error())
		return
	}

	fs, err := NewFileScanner(testFile)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	defer fs.Close()

	//пустой слайс или массив из одного нила
	fs.Scanner = fs.GetScanner()

	for fs.Scanner.Scan() {
		locs := fs.Scanner.Text()
		//log.Info("locs: %s", locs)
		time.Sleep(1000 * time.Millisecond)
		reqInJSON, err := MakeReq(locs, clientID, log)
		if err != nil {
			log.Error("Could not get coordinates in JSON: wrong format")
			continue
		}
		//log.Info("reqInJSON: %s", *reqInJSON)

		time.Sleep(1000 * time.Millisecond)

		err = conn.Send(*destination, "text/json", []byte(*reqInJSON), nil...)
		if err != nil {
			log.Error("failed to send to server", err)
			return
		}

	}
}

func recvMessages(subscribed chan bool) {
	defer func() {
		stop <- true
	}()
	conn, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		println("cannot connect to server", err.Error())
		return
	}
	log.Info("Subscribing to %s", *queueFormat+clientID)

	sub, err := conn.Subscribe(*queueFormat+clientID, stomp.AckAuto)
	if err != nil {
		println("cannot subscribe to", *queueFormat+clientID, err.Error())
		return
	}
	close(subscribed)

	for {
		msg := <-sub.C
		if msg == nil {
			log.Warn("Got empty message; ignore")
			return
		}
		actualText := string(msg.Body)
		log.Info("Got message:", actualText)
	}
}

type Req struct {
	Lat      float64 `json: Lat`
	Lon      float64 `json:Lon`
	Zoom     int     `json:Zoom`
	ClientID string  `json:ClientID`
}

func (r *Req) getLocationJSON() (string, error) {

	dataJSON, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(dataJSON), nil
}

//type error interface {
//	Error() string
//}

func MakeReq(parameters, clientID string, log l4g.Logger) (reqInJSON *string, err error) {

	locSlice := strings.Split(parameters, ",")
	r := Req{}
	r.Lat, err = strconv.ParseFloat(locSlice[0], 32)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	r.Lon, err = strconv.ParseFloat(locSlice[1], 32)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	r.Zoom, err = strconv.Atoi(locSlice[2])
	if err != nil {
		log.Error(err)
		return nil, err
	}
	r.ClientID = clientID

	jsonReq, err := r.getLocationJSON()
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return &jsonReq, nil
}

type fileScanner struct {
	File    *os.File
	Scanner *bufio.Scanner
	Reader  *bufio.Reader
}

func NewFileScanner(fileName string) (*fileScanner, error) {
	tmpFile, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	fs := fileScanner{File: tmpFile}
	return &fs, nil
}

func (f *fileScanner) Close() error {
	return f.File.Close()
}

/*func (f *fileScanner) GetReader() *bufio.Reader {
	if f.Reader == nil {
		f.Reader = bufio.NewReader(f.File)
	}
	return f.Reader
}
*/

func (f *fileScanner) GetScanner() *bufio.Scanner {
	if f.Scanner == nil {
		f.Scanner = bufio.NewScanner(f.File)
		//f.Scanner.Split(bufio.ScanLines)
	}
	return f.Scanner
}

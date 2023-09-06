package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lithammer/shortuuid"
)

const (
	Port = 3000
)

func main() {
	fmt.Println("Starting")

	conns := make(map[string]*connection, 0)
	outChan := make(chan message)

	mainCssServer := cssServer{
		conns:        &conns,
		outChan:      outChan,
		messages:     make([]string, 0),
		messageLimit: 10,
		buttons:      []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z", "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z", "_", "-"},
	}

	go messageSender(&mainCssServer)
	//go sendRandomMessages(outChan)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqHandler(w, r, &mainCssServer)
	})
	s := &http.Server{
		Addr:         ":" + strconv.Itoa(Port),
		Handler:      h,
		ReadTimeout:  10 * time.Minute,
		WriteTimeout: 10 * time.Minute,
	}
	fmt.Println("Running server on port " + strconv.Itoa(Port))
	s.ListenAndServe()

	fmt.Println("Finished")
}

func sendRandomMessages(out chan message) {
	for i := 0; ; i++ {
		fmt.Println("Sending new message")
		out <- message{
			senderId: "test",
			data:     "Message #" + strconv.Itoa(i),
		}
		time.Sleep(3 * time.Second)
	}
}

func reqHandler(w http.ResponseWriter, r *http.Request, s *cssServer) {
	url := r.RequestURI[1:]
	idx := strings.IndexRune(url, '/')
	firstPath := url
	if idx >= 0 {
		firstPath = url[:idx]
	}
	switch firstPath {
	case "":
		respondHomePage(w, r)
	case "connect":
		newConnection(w, r, s)
	case "letter":
		newLetter(w, r, s)
	case "send":
		newSendMessage(w, r, s)
	default:
		respond404(w, r)
	}
}

func respondHomePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Welcome to the CSS only web chat.\n" +
		"Go to /connect/<some-username> to join.")
}

func respond404(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "404- "+r.RequestURI+" not found.")
}

func messageSender(s *cssServer) {
	for m := range s.outChan {
		s.messages = append(s.messages, m.data)
		if len(s.messages) > s.messageLimit {
			s.messages = s.messages[len(s.messages)-s.messageLimit:]
		}
		for _, c := range *s.conns {
			//if c.id != m.senderId {
			c.in <- "New message"
			//}
		}
	}
}

func newConnection(w http.ResponseWriter, r *http.Request, s *cssServer) {
	if len(r.RequestURI) <= len("/connect/") {
		fmt.Fprint(w, "Invalid username")
		return
	}
	name := r.RequestURI[len("/connect/"):]
	c := connection{
		id:          generateId(),
		username:    name,
		in:          make(chan string),
		chunkId:     0,
		currMessage: "",
	}
	(*s.conns)[c.id] = &c
	connHandler(w, r, &c, s)
}

func generateId() string {
	return shortuuid.New()
}

func connHandler(w http.ResponseWriter, r *http.Request, conn *connection, s *cssServer) {
	sendInitial(w, s, conn)

	flushWriter(w)

	s.outChan <- message{
		senderId: conn.id,
		data:     conn.username + " joined the chat.",
	}

	for range conn.in {
		sendPageChunk(w, s, (*s.conns)[conn.id])
		flushWriter(w)
		(*conn).chunkId++
	}

	fmt.Fprintln(w, "End")
}

func sendInitial(w http.ResponseWriter, s *cssServer, conn *connection) {
	w.WriteHeader(200)
	fmt.Fprint(w, "<!DOCTYPE html><html><head><title>CSS Web Chat!</title></head>")
	fmt.Fprint(w, "<body><h1>CSS Web Chat!</h1><style>.messages{height:500px;overflow-y:scroll;}</style>")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Transfer-Encoding", "chunked")
	sendPageChunk(w, s, conn)
}

func sendPageChunk(w http.ResponseWriter, s *cssServer, conn *connection) {
	fmt.Println("Username: " + conn.username)

	fmt.Fprint(w, "<div id=\"chunk-"+strconv.Itoa(conn.chunkId)+"\"><div class=\"messages\">")
	for _, m := range s.messages {
		fmt.Fprint(w, "<p>"+m+"</p>")
	}
	fmt.Fprint(w, "</div><br/><p>"+conn.username+": "+strings.ReplaceAll(conn.currMessage, "_", " ")+"</p><div>")
	for _, b := range s.buttons {
		bSymbol := b
		if b == "-" {
			b = "Backspace"
		}
		if b == "_" {
			b = "Space"
		}
		fmt.Fprint(w, "<button class=\""+conn.currMessage+bSymbol+"\">"+b+"</button>")
		if b == "Z" || b == "z" || b == "Backspace" {
			fmt.Fprint(w, "<br/>")
		}
	}
	fmt.Fprint(w, "<button class=\"send\">Send</button>")
	fmt.Fprint(w, "</div></div>")
	fmt.Fprint(w, "<style>#chunk-"+strconv.Itoa(conn.chunkId-1)+"{display:none}")
	for _, b := range s.buttons {
		url := "/letter/" + conn.id + "/" + strconv.Itoa(conn.messageId) + "/" + conn.currMessage + b
		fmt.Fprint(w, "#chunk-"+strconv.Itoa(conn.chunkId)+" ."+conn.currMessage+b+":active{background:url("+url+")}")
	}
	sendUrl := "/send/" + conn.id + "/" + strconv.Itoa(conn.messageId)
	fmt.Fprint(w, "#chunk-"+strconv.Itoa(conn.chunkId)+" .send:active{background:url("+sendUrl+")}")
	fmt.Fprint(w, "</style>")
}

func newLetter(w http.ResponseWriter, r *http.Request, s *cssServer) {
	path := strings.Split(r.RequestURI, "/")
	conn := (*s.conns)[path[2]]
	if strings.HasSuffix(path[4], "-") {
		if len(path[4]) <= 1 {
			conn.currMessage = ""
		} else {
			conn.currMessage = path[4][:len(path[4])-2]
		}
	} else {
		conn.currMessage = path[4]
	}

	conn.in <- "letter"
}

func newSendMessage(w http.ResponseWriter, r *http.Request, s *cssServer) {
	fmt.Println("New send: " + r.RequestURI)
	path := strings.Split(r.RequestURI, "/")
	conn := (*s.conns)[path[2]]
	if conn.currMessage == "" {
		return
	}
	s.outChan <- message{
		senderId: conn.id,
		data:     conn.username + ": " + strings.ReplaceAll(conn.currMessage, "_", " "),
	}
	conn.currMessage = ""
	conn.messageId++
}

func flushWriter(w http.ResponseWriter) bool {
	f, ok := w.(http.Flusher)
	if ok {
		f.Flush()
	}
	return ok
}

type cssServer struct {
	server       http.Server
	conns        *map[string]*connection
	outChan      chan message
	messages     []string
	messageLimit int
	buttons      []string
}

type message struct {
	senderId string
	data     string
}

type connection struct {
	id          string
	username    string
	in          chan string
	chunkId     int
	currMessage string
	messageId   int
}

package main

import (
	"fmt"
	"net/http"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/gorilla/websocket"
	"encoding/base64"
)

var upgrader = websocket.Upgrader{}

var encoding = base64.RawStdEncoding

func openConnection(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	r.Header.Del("Origin")
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.WithError(err).Errorf("Couldn't upgrade websocket")
		return
	}
	logrus.Infof("Accepted websocket connection")
	remoteConn, err := net.Dial("tcp", host)
	if err != nil {
		logrus.WithError(err).Errorf("Couldn't connection to host %s", host)
		return
	}
	logrus.Infof("Connected to %s", host)

	go func() {
		for {
			b := make([]byte, 1000)
			n, err := remoteConn.Read(b)
			if err != nil {
				logrus.WithError(err).Errorf("[TCP->WS]Failed to read")
				_ = wsConn.Close()
				_ = remoteConn.Close()
				return
			}
			newB := make([]byte, encoding.EncodedLen(n))
			encoding.Encode(newB, b[:n])
			err = wsConn.WriteMessage(websocket.TextMessage, newB)
			if err != nil {
				logrus.WithError(err).Errorf("[TCP->WS] Failed to write")
				_ = wsConn.Close()
				_ = remoteConn.Close()
				return
			}
		}
	}()
	go func() {
		for {

			_, message, err := wsConn.ReadMessage()
			if err != nil {
				logrus.WithError(err).Errorf("[WS->TCP]Failed to read")
				_ = wsConn.Close()
				_ = remoteConn.Close()
				return
			}
			logrus.Infof("%02X", message)
			newB := make([]byte, encoding.DecodedLen(len(message)))
			n, err := encoding.Decode(newB, message)
			if err != nil {
				logrus.WithError(err).Errorf("[WS->TCP]Failed to decode")
				_ = wsConn.Close()
				_ = remoteConn.Close()
				return
			}

			offset := 0
			for offset < n {
				written, err := remoteConn.Write(newB[offset:n])
				if err != nil {
					logrus.WithError(err).Errorf("[WS->TCP]Failed to Write")
					return
				}
				offset += written
			}
		}
	}()
}

func main() {
	http.HandleFunc("/socket", openConnection)
	err := http.ListenAndServe(fmt.Sprintf(":%d", 5678), nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

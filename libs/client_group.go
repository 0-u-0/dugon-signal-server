package libs

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ClientGroup struct {
	clients map[*Client]bool

	register   chan *Client
	unregister chan *Client

	nc *nats.EncodedConn
}

func NewClientGroup(natsUrls []string) *ClientGroup {
	g := &ClientGroup{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}

	natsUrl := strings.Join(natsUrls, " ,")
	nc, err := nats.Connect(natsUrl)
	if err != nil {

	}
	c, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {

	}
	g.nc = c
	return g
}

func (g *ClientGroup) Run() {
	for {
		select {
		case client := <-g.register:
			g.clients[client] = true
		case client := <-g.unregister:
			if _, ok := g.clients[client]; ok {
				fmt.Printf("%s client close\n", client.tokenId)
				delete(g.clients, client)
			}
		}
	}
}

type wsHandler struct {
	clientGroup *ClientGroup
}

func InitWsServer(g *ClientGroup, port int, httpsEnable bool,
	crtPath string, keyPath string) {
	address := fmt.Sprintf(":%d", port)

	s := &http.Server{
		Addr: address,
		Handler: wsHandler{
			clientGroup: g,
		},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if httpsEnable {
		fmt.Println(s.ListenAndServeTLS(crtPath, keyPath))
	} else {
		fmt.Println(s.ListenAndServe())
	}
}

func (handler wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlQuery := r.URL.RawQuery
	values, _ := url.ParseQuery(urlQuery)
	paramsEncodedArr, ok := values["params"]
	if ok {
		queryByte, _ := b64.URLEncoding.DecodeString(paramsEncodedArr[0])
		var params requestParams
		err := json.Unmarshal(queryByte, &params)
		if err != nil {
			//TODO(CC): response error
			fmt.Println("error:", err)
			return
		}
		upgrade := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		}

		header := http.Header{}
		conn, err := upgrade.Upgrade(w, r, header)

		if err != nil {
			fmt.Println("error:", err)
			return
		}

		client := newClient(handler.clientGroup, conn, params.TokenId, params.SessionId, params.Metadata)
		handler.clientGroup.register <- client

		go client.WritePump()
		go client.readPump()
	}

}

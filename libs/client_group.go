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

type MediaServer struct {
	Id      string `json:"id"`
	Area    string `json:"area"`
	Host    string `json:"host"`
	Name    string `json:"name"`
	isAlive bool
}

type ClientGroup struct {
	clients map[*client]bool

	register   chan *client
	unregister chan *client

	nc *nats.EncodedConn

	mediaServers map[string]*MediaServer
}

func NewClientGroup(natsUrls []string) *ClientGroup {
	g := &ClientGroup{
		clients:      make(map[*client]bool),
		register:     make(chan *client),
		unregister:   make(chan *client),
		mediaServers: make(map[string]*MediaServer),
	}

	natsUrl := strings.Join(natsUrls, " ,")
	//FIXME: add reconnect
	nc, err := nats.Connect(natsUrl)
	if err != nil {
		Log.Fatalf("Nat connect error  %w\n", err)
	}
	c, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		Log.Fatalf("Nat json connect error  %w\n", err)
	}
	g.nc = c

	g.nc.Subscribe("media@heartbeat", func(m *nats.Msg) {
		//fmt.Println(string(m.Data))
		var info = &MediaServer{}
		err := json.Unmarshal(m.Data, info)
		if err != nil {
			Log.Warnf("Heartbeat json decode error : %w\n", err)
		}

		if media, ok := g.mediaServers[info.Id]; ok {
			//keepalive
			media.isAlive = true
		} else {
			Log.Infof("Media server [%s] registered\n", info.Id)
			info.isAlive = true
			g.mediaServers[info.Id] = info
		}
	})

	return g
}

func (g *ClientGroup) Run() {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()

	for {
		select {
		case client := <-g.register:
			g.clients[client] = true
		case client := <-g.unregister:
			if _, ok := g.clients[client]; ok {
				Log.Debugf("%s client close\n", client.tokenId)
				delete(g.clients, client)
			}
		case <-t.C:
			for i, e := range g.mediaServers {
				if e.isAlive {
					e.isAlive = false
				} else {
					Log.Debugf("media server {%s} died\n", e.Name)
					delete(g.mediaServers, i)
				}
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
		Log.Fatal(s.ListenAndServeTLS(crtPath, keyPath))
	} else {
		Log.Fatal(s.ListenAndServe())
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
			Log.Warnf("Json decode error : %w\n",err)
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
			Log.Warnf("Websocket err : %w\n", err)
			return
		}

		client := newClient(handler.clientGroup, conn, params.TokenId, params.SessionId, params.Metadata)
		handler.clientGroup.register <- client

		go client.writePump()
		go client.readPump()
		go client.processPump()
	}

}

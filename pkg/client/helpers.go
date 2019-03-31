package client

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

func getLoginURL(serverDomain string, useSSL bool) string {
	protocol := "http"
	if useSSL {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s/%s", protocol, serverDomain, loginURLPath)
}

func getWebsocketURL(serverDomain string, useSSL bool) string {
	protocol := "ws"
	if useSSL {
		protocol = "wss"
	}
	return fmt.Sprintf("%s://%s/%s", protocol, serverDomain, wsURLPath)
}

// getSendRequest returns the request that is send by the admin clients
func getSendRequest(serverDomain string, useSSL bool) (r *http.Request) {
	protocol := "http"
	if useSSL {
		protocol = "https"
	}

	r, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s://%s/%s", protocol, serverDomain, "rest/agenda/item/1/"),
		strings.NewReader(`
			{"id":1,"item_number":"","title":"foo1","list_view_title":"foo1",
			"comment":"test","closed":false,"type":1,"is_hidden":false,"duration":null,
			"speaker_list_closed":false,"content_object":{"collection":"topics/topic",
			"id":1},"weight":10000,"parent_id":null,"parentCount":0,"hover":true}`),
	)
	if err != nil {
		log.Fatalf("Coud not build the request, %s", err)
	}
	r.Close = true
	return r
}

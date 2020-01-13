package main

import (
	"log"
	"net/http"
)

func main(){
	address := "localhost:5000"
	http.HandleFunc("/", home)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatalf("Error of listen server: %s", err)
	}

}

func home (w http.ResponseWriter, r *http.Request){
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
	http.ServeFile(w, r, "start_page.html")
}

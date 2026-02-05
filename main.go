package main

func main() {
	// server := NewServer("127.0.0.1:4000")
	// router := NewRouter(server)
	// _, err := NewPostgresStore("user=postgres password=dummy dbname=postgres sslmode=disable")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// server.Start(router)

	crawler := NewCrawler()
	crawler.Crawl()
}

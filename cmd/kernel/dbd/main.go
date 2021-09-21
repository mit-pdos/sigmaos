package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("mysql", "sigma:sigmaos@/books")
	if err != nil {
		log.Fatal(err)
	}
	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}
	fmt.Println("Connected!")
	books, err := booksByAuthor("Homer")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Books found: %v\n", books)
}

type Book struct {
	ID     int64
	Title  string
	Author string
	Price  float32
}

func booksByAuthor(name string) ([]Book, error) {
	var books []Book

	rows, err := db.Query("SELECT * FROM book WHERE author = ?", name)
	if err != nil {
		return nil, fmt.Errorf("booksByAuthor %q: %v", name, err)
	}
	defer rows.Close()
	for rows.Next() {
		var alb Book
		if err := rows.Scan(&alb.ID, &alb.Title, &alb.Author, &alb.Price); err != nil {
			return nil, fmt.Errorf("booksByAuthor %q: %v", name, err)
		}
		books = append(books, alb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("booksByAuthor %q: %v", name, err)
	}
	return books, nil
}

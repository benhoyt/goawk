package main

import (
	"encoding/csv"
	"log"
	"os"
	"strconv"
)

func main() {
	writer := csv.NewWriter(os.Stdout)
	for i := 0; i < 3514073; i++ { // will create a ~1GB file
		err := writer.Write([]string{
			strconv.Itoa(i),
			"foo",
			"bob@example.com",
			"simple,quoted",
			"quoted string with \" in it",
			"0123456789",
			"9876543210",
			"The quick brown fox jumps over the lazy dog",
			"",
			"final field",
			strconv.Itoa(i),
			"foo",
			"bob@example.com",
			"simple,quoted",
			"quoted string with \" in it",
			"0123456789",
			"9876543210",
			"The quick brown fox jumps over the lazy dog",
			"",
			"final field",
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	writer.Flush()
	if writer.Error() != nil {
		log.Fatal(writer.Error())
	}
}

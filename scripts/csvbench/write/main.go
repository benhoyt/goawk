package main

import (
	"encoding/csv"
	"log"
	"os"
	"strconv"
)

func main() {
	writer := csv.NewWriter(os.Stdout)
	for i := 0; i < 10000000; i++ {
		err := writer.Write([]string{strconv.Itoa(i), "foo", "bob@example.com", "quoted,string", "final field"})
		if err != nil {
			log.Fatal(err)
		}
	}
	writer.Flush()
	if writer.Error() != nil {
		log.Fatal(writer.Error())
	}
}

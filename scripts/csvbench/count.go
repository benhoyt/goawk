package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
)

func main() {
	reader := csv.NewReader(bufio.NewReader(os.Stdin))
	lines, fields := 0, 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		lines++
		fields += len(row)
	}
	fmt.Println(lines, fields)
}

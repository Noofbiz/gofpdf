/*
 * Copyright (c) 2017 Jerry Caligiure (Gmail: caligiure.ja)
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package gofpdf

import (
	"fmt"
	"io"
	"os"
)

//Load takes in a pdf at path filePath and returns a *Fpdf
//
// filePath can be either an absolute file path or relative to the running
//  program.
//
// *Fpdf is as close to the original pdf as possible. Probably needs more real
// world testing to get everything right. Feel free to improve this or propose
// improvements on the github.
func Load(filePath string) (f *Fpdf, err error) {
	f = New("P", "mm", "A4", "")
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Unable to open file at path %v. \r\n Error: %v", filePath, err.Error())
		return
	}
	defer file.Close()
	buf := make([]byte, 2)
	var currentSpot int64
	currentLine := make([]byte, 10)
	lines := make([][]byte, 10)
	for {
		_, err = file.ReadAt(buf, currentSpot)
		if err != nil {
			if err == io.EOF {
				fmt.Println("At end of file!")
				break
			}
			fmt.Printf("Error reading file! Error: %v", err.Error())
			return
		}

		if rune(buf[0]) == '\r' || rune(buf[0]) == '\n' {
			lines = append(lines, currentLine)
			currentLine = make([]byte, 10)
			if rune(buf[1]) == '\n' {
				currentSpot++
			}
		} else {
			currentLine = append(currentLine, buf[0])
		}

		currentSpot++
	}

	for _, line := range lines {
		fmt.Printf("%v \r\n", string(line))
	}

	return
}

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
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
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
	var reader pdfReader

	if err = reader.openFile(filePath); err != nil {
		return nil, err
	}

	if err = reader.parseXRefTable(); err != nil {
		return nil, err
	}

	f, err = reader.buildFpdf()
	if err != nil {
		return nil, err
	}

	if err = reader.closeFile(); err != nil {
		return nil, err
	}

	return f, nil
}

type pdfReader struct {
	file          *os.File
	recentReading []byte
	cursorAt      int64
	recentDict    map[string]interface{}
	pdfVersion    byte
	inStrEsc      bool
	xrefOffset    int64
	xrefTable     xref
}

var whiteSpaceChars = []byte{0, //Null
	9,  //Horizontal Tab
	10, //Line Feed
	12, //Form Feed
	13, //Carriage return
	32} //Space

var delimiterChars = []byte{40, //left parenthesis
	41,  //right parenthesis
	60,  //less than sign
	62,  //greater than sign
	91,  //left square bracket
	93,  //right square bracket
	123, //left curly brace
	125, //right curly brace
	47,  //Solidus
	37}  //percent

func (r *pdfReader) openFile(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	r.file = f

	if err = r.pdfReadAt(0, false, true); err != nil {
		return err
	}
	if !bytes.HasPrefix(r.recentReading, []byte("%PDF-1.")) {
		return errors.New("malformed PDF header")
	}
	r.pdfVersion = r.recentReading[7]
	if r.pdfVersion < '0' || r.pdfVersion > '7' {
		return errors.New("version incompatible with this reader")
	}

	if err = r.pdfReadAt(r.pdfSize(), true, true); err != nil {
		return err
	}
	if !bytes.HasPrefix(r.recentReading, []byte("FOE%%")) {
		fmt.Println(r.recentReading)
		return errors.New("malformed PDF footer")
	}
	if err = r.pdfReadAt(r.cursorAt, true, false); err != nil {
		return err
	}
	r.xrefOffset, err = strconv.ParseInt(string(reverseByteSlice(r.recentReading)),
		10, 64)
	if err != nil {
		return err
	}

	r.recentDict = make(map[string]interface{})

	return nil
}

type xref struct {
	objStart, objEnd int
	objTable         []xrefObjData
	trailer          map[string]interface{}
}

type xrefObjData struct {
	offset, generationNumber, objNum int
	inUse                            bool
}

type objReference struct {
	objNum, generationNum int
}

func (r *pdfReader) closeFile() error {
	return r.file.Close()
}

func (r *pdfReader) pdfSize() int64 {
	fileInfo, _ := r.file.Stat()
	return fileInfo.Size()
}

func (r *pdfReader) pdfReadAt(readOffset int64, readBackwards, headfoot bool) error {
	var reading []byte
	r.cursorAt = readOffset
	buf := make([]byte, 1)
	for {
		_, err := r.file.ReadAt(buf, r.cursorAt)
		if err != nil && err != io.EOF {
			return err
		}

		ck := isInByteArr(buf[0], delimiterChars)
		if !headfoot {
			if ck && len(reading) != 0 {
				r.recentReading = reading
				return nil
			}
		}

		if readBackwards {
			r.cursorAt--
		} else {
			r.cursorAt++
		}

		ck = isInByteArr(buf[0], whiteSpaceChars)
		if ck && len(reading) == 0 {
			continue
		}
		if ck {
			r.recentReading = reading
			return nil
		}

		reading = append(reading, buf[0])

		if err == io.EOF {
			r.recentReading = reading
			return nil
		}
	}
}

func isInByteArr(b byte, arr []byte) bool {
	for _, val := range arr {
		if b == val {
			return true
		}
	}
	return false
}

func reverseByteSlice(in []byte) []byte {
	for i := 0; i < len(in)/2; i++ {
		j := len(in) - i - 1
		in[i], in[j] = in[j], in[i]
	}
	return in
}

const (
	objNumStart int = iota
	objNumEnd
	objTabCol1
	objTabCol2
	objTabCol3
	trailerPart
)

func (r *pdfReader) parseXRefTable() error {
	err := r.pdfReadAt(r.xrefOffset, false, false)
	if err != nil {
		return err
	}

	if bytes.Compare(r.recentReading, []byte("xref")) != 0 {
		errors.New("malformed pdf: xref offset was incorrect")
	}

	part := objNumStart
	var objRow xrefObjData
	var objNum int

	for bytes.Compare(r.recentReading, []byte("startxref")) != 0 {
		err = r.pdfReadAt(r.cursorAt, false, false)
		if err != nil {
			return err
		}
		if bytes.Compare(r.recentReading, []byte("trailer")) == 0 {
			part = trailerPart
		}
		switch part {
		case objNumStart:
			start, err := strconv.Atoi(string(r.recentReading))
			if err != nil {
				return err
			}
			r.xrefTable.objStart = start
			objNum = r.xrefTable.objStart
			part = objNumEnd
		case objNumEnd:
			end, err := strconv.Atoi(string(r.recentReading))
			if err != nil {
				return err
			}
			r.xrefTable.objEnd = end
			part = objTabCol1
		case objTabCol1:
			objRow = xrefObjData{}
			objRow.objNum = objNum
			objNum++
			offset, err := strconv.Atoi(string(r.recentReading))
			if err != nil {
				return err
			}
			objRow.offset = offset
			part = objTabCol2
		case objTabCol2:
			genNum, err := strconv.Atoi(string(r.recentReading))
			if err != nil {
				return err
			}
			objRow.generationNumber = genNum
			part = objTabCol3
		case objTabCol3:
			if bytes.Compare(r.recentReading, []byte("f")) == 0 {
				objRow.inUse = false
			} else if bytes.Compare(r.recentReading, []byte("n")) == 0 {
				objRow.inUse = true
			} else {
				return errors.New("malformed xref table third col can be only f or n")
			}
			r.xrefTable.objTable = append(r.xrefTable.objTable, objRow)
			part = objTabCol1
		case trailerPart:
			r.pdfReadAt(r.cursorAt, false, false)
			err := r.parsePDFDict()
			if err != nil {
				return err
			}
			r.xrefTable.trailer = r.recentDict
		}
	}
	return nil
}

func (r *pdfReader) parsePDFDict() error {
	if !bytes.HasPrefix(r.recentReading, []byte("<")) {
		return errors.New("expected pdf dictionary")
	}

	r.pdfReadAt(r.cursorAt, false, false)
	if !bytes.HasPrefix(r.recentReading, []byte("<")) {
		return errors.New("expected pdf dictionary")
	}

	r.pdfReadAt(r.cursorAt, false, false)
	atKey := true
	cK := ""
	for !bytes.HasSuffix(r.recentReading, []byte(">")) {
		if atKey {
			key, err := r.parsePDFObject()
			if err != nil {
				return err
			}
			cK = key.(string)
			atKey = false
		} else {
			val, err := r.parsePDFObject()
			if err != nil {
				return err
			}
			r.recentDict[cK] = val
			atKey = true
		}
		r.pdfReadAt(r.cursorAt, false, false)
	}

	r.pdfReadAt(r.cursorAt, false, false)
	if !bytes.HasPrefix(r.recentReading, []byte(">")) {
		return errors.New("pdf dictionary ending malformed")
	}
	r.pdfReadAt(r.cursorAt, false, false)
	return nil
}

func (r *pdfReader) parsePDFObject() (interface{}, error) {
	switch {
	case bytes.HasPrefix(r.recentReading, []byte("/")):
		return string(r.recentReading[1:len(r.recentReading)]), nil
	case byte('0') <= r.recentReading[0] && byte('9') >= r.recentReading[0]:
		peek, err := r.peek()
		if err != nil {
			return nil, err
		}
		if isInByteArr(peek[0], delimiterChars) {
			if bytes.ContainsRune(r.recentReading, '.') {
				return strconv.ParseFloat(string(r.recentReading), 32)
			}
			return strconv.Atoi(string(r.recentReading))
		}
		var retObj objReference
		num, err := strconv.Atoi(string(r.recentReading))
		if err != nil {
			return nil, err
		}
		retObj.objNum = num
		r.pdfReadAt(r.cursorAt, false, false)
		gen, err := strconv.Atoi(string(r.recentReading))
		if err != nil {
			return nil, err
		}
		retObj.generationNum = gen
		r.pdfReadAt(r.cursorAt, false, false)
		return retObj, nil
	}

	return nil, nil
}

func (r *pdfReader) peek() ([]byte, error) {
	saveLoc := r.cursorAt
	saveRead := r.recentReading

	err := r.pdfReadAt(r.cursorAt, false, false)
	if err != nil {
		return nil, err
	}

	retRead := r.recentReading
	r.cursorAt = saveLoc
	r.recentReading = saveRead
	return retRead, nil
}

func (r *pdfReader) buildFpdf() (f *Fpdf, err error) {
	fmt.Println(r.xrefTable)
	return f, err
}

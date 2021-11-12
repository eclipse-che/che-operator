//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"bufio"
)

var (
	licenseTemplatePath string
)

func init() {
	flag.StringVar(&licenseTemplatePath, "f", "", "License header template")
}

type CommentDelimiters struct {
	Start     string
	Delimiter string
	End       string
}

func main() {
	flag.Parse()
	if len(licenseTemplatePath) == 0 {
		fmt.Println("Set up license template using flag '-f'")
		os.Exit(1)
	}
	if flag.NArg() == 0 {
		flag.Usage()
		fmt.Println("There is no arguments with file paths!")
		os.Exit(1)
	}

	templLines := readTemplate(licenseTemplatePath)
	templSize := len(templLines)
	rxp, err := regexp.Compile("(\\s*Copyright \\(c\\)\\s+)([0-9]{4}(-{1}[0-9]{4})?)(\\s+.*)")
	if err != nil {
		log.Fatal(err.Error())
	}
	// fmt.Println(templLines)

	for _, file := range flag.Args() {
		// fmt.Printf("file path %s", file)
		commentDelms := getCommentDelimiters(file)
		// fmt.Println(commentDelms)
		fileContent := readNLinesFromFile(file, templSize)
		// fmt.Println(fileContent)
		for i, line := range templLines {
			var licenseLine string
			switch i {
			case 0:
				licenseLine = commentDelms.Start + line
			case len(templLines) - 2:
				licenseLine = commentDelms.End + line
			case len(templLines) - 1:
				// do nothing, empty line
			default:
				if len(line) > 0 {
					licenseLine = commentDelms.Delimiter + " " + line
				} else {
					licenseLine = commentDelms.Delimiter
				}
			}
			
			// Compare 'Copyright (c).*' line, but respect years provided by developer
			if rxp.MatchString(licenseLine) {
				licenseLineWithoutYears := rxp.ReplaceAllString(licenseLine, "$1$4")
				fileContentLineWithoutYears := rxp.ReplaceAllString(fileContent[i], "$1$4")
				if licenseLineWithoutYears != fileContentLineWithoutYears {
					fmt.Printf("Bad formatted license header for file %s \n", file)
					break
				}
				continue
			}
			if licenseLine != fileContent[i] {
				fmt.Printf("Bad formatted license header for file %s \n", file)
				break
			}
		}
	}
}

func readTemplate(filePath string) []string {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Unable to read template file %s", filePath)
	}

	return strings.Split(string(content), "\n")
}

// readNLinesFromFile read N lines from file, but skips first empty lines.
func readNLinesFromFile(filePath string, n int) []string {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Unable to open file %s", err.Error())
	}

	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Unable to close file %s. Cause: %s ", filePath, err.Error())
		}
	}()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var lines []string
	for i := 0; scanner.Scan() && i < n; {
		if strings.HasPrefix(scanner.Text(), "#!") && filepath.Ext(filePath) == ".sh" {
			// skip line with shell binary definition f.e.: '#!/bin/bash'
			continue
		}
		if len(scanner.Text()) > 0 || len(lines) > 0 {
			lines = append(lines, scanner.Text())
			i++
		}
	}
	return lines
}

func getCommentDelimiters(file string) *CommentDelimiters {
	fileExtension := filepath.Ext(file)
	switch fileExtension {
	case ".sh":
		return &CommentDelimiters{"#", "#", "#"}
	case ".go":
		return &CommentDelimiters{"//", "//", "//"}
	default:
		log.Fatalf("Unsupported file extension:")
	}
	return nil
}

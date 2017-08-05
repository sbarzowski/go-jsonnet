/*
Copyright 2016 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jsonnet

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

type FileImporter struct {
	// TODO(sbarzowski) fill it in
	JPaths []string
}

func tryPath(dir, importedPath string) (found bool, content []byte, foundHere string, err error) {
	var absPath string
	if path.IsAbs(importedPath) {
		absPath = importedPath
	} else {
		absPath = path.Join(dir, importedPath)
	}
	content, err = ioutil.ReadFile(absPath)
	if os.IsNotExist(err) {
		return false, nil, "", nil
	}
	return true, content, absPath, err
}

func (importer *FileImporter) Import(dir, importedPath string) (*ImportedData, error) {
	found, content, foundHere, err := tryPath(dir, importedPath)
	if err != nil {
		return nil, err
	}

	for i := 0; !found && i < len(importer.JPaths); i++ {
		found, content, foundHere, err = tryPath(importer.JPaths[i], importedPath)
		if err != nil {
			return nil, err
		}
	}

	return &ImportedData{content: string(content), foundHere: foundHere}, nil
}

type MemoryImporter struct {
	data map[string]string
}

func (importer *MemoryImporter) Import(dir, importedPath string) (*ImportedData, error) {
	if content, ok := importer.data[importedPath]; ok {
		return &ImportedData{content: content, foundHere: importedPath}, nil
	}
	return nil, fmt.Errorf("Import not available %v", importedPath)
}

func snippetToAST(filename string, snippet string) (astNode, error) {
	tokens, err := lex(filename, snippet)
	if err != nil {
		return nil, err
	}
	ast, err := parse(tokens)
	if err != nil {
		return nil, err
	}
	// fmt.Println(ast.(dumpable).dump())
	err = desugarFile(&ast)
	if err != nil {
		return nil, err
	}
	err = analyze(ast)
	if err != nil {
		return nil, err
	}
	return ast, nil
}

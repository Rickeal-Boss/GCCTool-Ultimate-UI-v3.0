package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	src := "C:/Users/16896/Desktop/job/深度搜索一下广州商学院正方教务系统新版api登录抢课go语言别人怎么做的.docx"
	dst := "C:/Users/16896/WorkBuddy/20260313161311/docx_unpacked"

	r, err := zip.OpenReader(src)
	if err != nil {
		fmt.Printf("Error opening zip: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dst, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			fmt.Printf("Error creating dir: %v\n", err)
			os.Exit(1)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			fmt.Printf("Error creating file: %v\n", err)
			os.Exit(1)
		}

		rc, err := f.Open()
		if err != nil {
			fmt.Printf("Error opening file in zip: %v\n", err)
			os.Exit(1)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			fmt.Printf("Error copying: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Extraction complete!")
}

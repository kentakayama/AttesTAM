/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
)

func echoUploadedFileAsDownload(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return fmt.Errorf("failed to parse form")
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return fmt.Errorf("file is required")
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read upload file")
	}

	filename := header.Filename
	if filename == "" {
		filename = "uploaded.bin"
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(filename))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
	return nil
}

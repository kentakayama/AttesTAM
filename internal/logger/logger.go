/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package logger

import (
	"io"
	"log/slog"
	"os"
)

func New() *slog.Logger {
	return NewWithWriter(os.Stdout, slog.LevelDebug)
}

func NewWithWriter(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
	}))
}

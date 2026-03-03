/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package logger

import "log/slog"

func New() *slog.Logger {
	return slog.Default()
}

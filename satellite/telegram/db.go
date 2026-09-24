// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package telegram

import (
	"context"
	"database/sql"
	"errors"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/satellitedb/dbaccess"
	"storj.io/storj/shared/tagsql"
)

// Persistence stores the Telegram chat connected by each console user.
type Persistence interface {
	// GetChatID returns the chat of the user, or 0 when the user has not connected one.
	GetChatID(ctx context.Context, user uuid.UUID) (int64, error)
	// SaveChatID connects the chat to the user, replacing the previous one.
	SaveChatID(ctx context.Context, user uuid.UUID, chatID int64) error
	// DeleteChatID disconnects the chat of the user.
	DeleteChatID(ctx context.Context, user uuid.UUID) error
}

// DB stores the Telegram chats in the telegram_chats table of the satellite database.
type DB struct {
	database tagsql.DB
}

var _ Persistence = (*DB)(nil)

// NewDB migrates the Telegram tables to the latest version and returns the store of the chats.
func NewDB(ctx context.Context, log *zap.Logger, m *Migration, db dbaccess.Access) (Persistence, error) {
	err := m.Migration.Run(ctx, log.Named("telegram-migration"))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &DB{
		database: db.GetDB(),
	}, nil
}

// GetChatID implements Persistence.
func (d *DB) GetChatID(ctx context.Context, user uuid.UUID) (chatID int64, err error) {
	defer mon.Task()(&ctx)(&err)

	err = d.database.QueryRowContext(ctx, `SELECT chat_id FROM telegram_chats WHERE user_id = $1`, user).Scan(&chatID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return chatID, Error.Wrap(err)
}

// SaveChatID implements Persistence. created_at is the time the current chat
// was connected, so it is refreshed when the chat is replaced.
func (d *DB) SaveChatID(ctx context.Context, user uuid.UUID, chatID int64) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = d.database.ExecContext(ctx, `
		INSERT INTO telegram_chats (user_id, chat_id) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET chat_id = EXCLUDED.chat_id, created_at = current_timestamp`,
		user, chatID)
	return Error.Wrap(err)
}

// DeleteChatID implements Persistence.
func (d *DB) DeleteChatID(ctx context.Context, user uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = d.database.ExecContext(ctx, `DELETE FROM telegram_chats WHERE user_id = $1`, user)
	return Error.Wrap(err)
}

package addressinf0db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	_ "modernc.org/sqlite"
	"sync"
	"time"
)

type AddressInf0DB struct {
	db             *sql.DB
	ctx            context.Context
	flipsideClient *rpc.Client
	updateLock     sync.Mutex
}

func Open(path string, flipsideAPIKey string) (addressInf0DB *AddressInf0DB, err error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?cache=shared", path))
	if err != nil {
		return nil, err
	}
	ctx := context.Background()

	const queryMaintenance = `CREATE TABLE IF NOT EXISTS maintenance (
	    chainID         INTEGER,
        maintenanceTime INTEGER,
		PRIMARY KEY (chainID)
	) WITHOUT ROWID;`
	if _, err = db.ExecContext(ctx, queryMaintenance); err != nil {
		_ = db.Close()
		return nil, err
	}

	const queryInfo = `CREATE TABLE IF NOT EXISTS info (
	    chainID      INTEGER,
		address      TEXT,
		name         TEXT,
		label        TEXT,
		labelType    TEXT,
		labelSubtype TEXT,
		PRIMARY KEY (chainID, address)
	) WITHOUT ROWID;`
	if _, err = db.ExecContext(ctx, queryInfo); err != nil {
		_ = db.Close()
		return nil, err
	}

	flipsideClient, err := rpc.Dial(flipsideEndpoint)
	if err != nil {
		return nil, err
	}
	flipsideClient.SetHeader("x-api-key", flipsideAPIKey)

	addressInf0DB = &AddressInf0DB{
		db:             db,
		flipsideClient: flipsideClient,
		ctx:            ctx,
	}

	return addressInf0DB, nil
}

func (a *AddressInf0DB) JournalMode() (journalMode string, err error) {
	if err = a.db.QueryRowContext(a.ctx, `PRAGMA journal_mode;`).Scan(&journalMode); err != nil {
		return "", err
	}

	return journalMode, nil
}

func (a *AddressInf0DB) Chains() (chains []uint64, err error) {
	const query = `SELECT chainID FROM maintenance;`

	rows, err := a.db.QueryContext(a.ctx, query)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	for rows.Next() {
		var chainID uint64
		if err = rows.Scan(&chainID); err != nil {
			return nil, err
		}
		chains = append(chains, chainID)
	}

	return chains, nil
}
func (a *AddressInf0DB) MaintenanceTime(chainID uint64) (maintenanceTime int64, err error) {
	const query = `SELECT maintenanceTime FROM maintenance WHERE chainID = ?;`

	if err = a.db.QueryRowContext(a.ctx, query, chainID).Scan(&maintenanceTime); errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}

	return maintenanceTime, err
}
func (a *AddressInf0DB) UpdateMaintenanceTime(chainID uint64) (err error) {
	const query = `INSERT INTO maintenance (chainID, maintenanceTime)
                   VALUES (?, ?)
                   ON CONFLICT(chainID)
                   DO UPDATE
                   SET maintenanceTime = excluded.maintenanceTime;`

	_, err = a.db.ExecContext(a.ctx, query, chainID, time.Now().Unix())

	return err
}

type AddressRecord struct {
	ChainID      uint64
	Address      common.Address
	Name         string
	Label        string
	LabelType    string
	LabelSubtype string
}

func (a *AddressInf0DB) GetAddressRecord(chainID uint64, address common.Address) (addressRecord *AddressRecord, err error) {
	const query = `SELECT name, label, labelType, labelSubtype FROM info WHERE chainID = ? and address = ?`

	var (
		method       string
		label        string
		labelType    string
		labelSubtype string
	)
	if err = a.db.QueryRowContext(a.ctx, query, chainID, address.Hex()).Scan(&method, &label, &labelType, &labelSubtype); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	addressRecord = &AddressRecord{
		ChainID:      chainID,
		Address:      address,
		Name:         method,
		Label:        label,
		LabelType:    labelType,
		LabelSubtype: labelSubtype,
	}

	return addressRecord, err
}
func (a *AddressInf0DB) AddressRecords(chainID uint64) (addressRecords int64, err error) {
	const query = `SELECT COUNT(*) FROM info WHERE chainID = ?`

	if err = a.db.QueryRowContext(a.ctx, query, chainID).Scan(&addressRecords); err != nil {
		return 0, err
	}

	return addressRecords, nil
}
func (a *AddressInf0DB) UpsertAddressRecords(addressRecords []*AddressRecord) (err error) {
	tx, err := a.db.BeginTx(a.ctx, nil)
	if err != nil {
		return err
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	const query = `INSERT INTO info (chainID, address, name, label, labelType, labelSubtype)
                   VALUES (?, ?, ?, ?, ?, ?)
                   ON CONFLICT (chainID, address)
                   DO UPDATE SET name = excluded.name, label = excluded.label, labelType = excluded.labelType, labelSubtype = excluded.labelSubtype;`
	stmt, err := tx.PrepareContext(a.ctx, query)
	if err != nil {
		return err
	}
	defer func(stmt *sql.Stmt) {
		_ = stmt.Close()
	}(stmt)

	for _, addressRecord := range addressRecords {
		if _, err = stmt.ExecContext(
			a.ctx,
			addressRecord.ChainID,
			addressRecord.Address.Hex(),
			addressRecord.Name,
			addressRecord.Label,
			addressRecord.LabelType,
			addressRecord.LabelSubtype,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

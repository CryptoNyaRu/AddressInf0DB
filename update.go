package addressinf0db

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"sync"
	"time"
)

const (
	flipsideEndpoint = "https://api-v2.flipsidecrypto.xyz/json-rpc"
	retryTimes       = 10
)

const (
	Updating = iota
	Successful
	Warning
	Failed
)

var queriesSlice = []uint64{1, 56, 8453}
var queriesMap = map[uint64]string{
	1:    `SELECT * FROM ethereum.core.dim_labels WHERE LABEL_SUBTYPE NOT IN ('general_contract', 'deposit_wallet', 'pool', 'token_contract', 'nf_token_contract') AND ADDRESS_NAME NOT IN ('coinbase');`,
	56:   `SELECT * FROM bsc.core.dim_labels WHERE LABEL_SUBTYPE NOT IN ('general_contract', 'deposit_wallet', 'pool', 'token_contract', 'nf_token_contract');`,
	8453: `SELECT * FROM base.core.dim_labels WHERE LABEL_SUBTYPE NOT IN ('general_contract', 'deposit_wallet', 'pool', 'token_contract', 'nf_token_contract');`,
}

type UpdateProgress struct {
	Status int
	Log    string
}

type Logger struct {
	Info    func(contents ...any)
	Success func(contents ...any)
	Warning func(contents ...any)
	Error   func(contents ...any)
}

func (a *AddressInf0DB) UpdateAsync() (updateProgress chan *UpdateProgress) {
	a.updateLock.Lock()

	updateProgress = make(chan *UpdateProgress, 256)

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(1)

	go func() {
		waitGroup.Wait()
		a.updateLock.Unlock()
	}()

	go func() {
		defer func() {
			close(updateProgress)
			waitGroup.Done()
		}()

		for _, chainID := range queriesSlice {
			updateProgress <- &UpdateProgress{
				Status: Updating,
				Log:    fmt.Sprintf("Updating chain id: %d", chainID),
			}

			queryRunID, err := a.createQueryRun(queriesMap[chainID])
			if err != nil {
				updateProgress <- &UpdateProgress{
					Status: Failed,
					Log:    fmt.Sprintf("Failed to create query run: %s, chain id: %d", err, chainID),
				}
				return
			}
			updateProgress <- &UpdateProgress{
				Status: Updating,
				Log:    fmt.Sprintf("Query run id got: %s, chain id: %d", queryRunID, chainID),
			}

			timeOut := time.After(time.Minute * 5)
		loop:
			for {
				select {
				case <-timeOut:
					updateProgress <- &UpdateProgress{
						Status: Failed,
						Log:    fmt.Sprintf("Query run time out: %s chain id: %d", queryRunID, chainID),
					}
					return

				default:
					if err = a.queryRunStatus(queryRunID); err != nil {
						time.Sleep(1 * time.Second)
						continue
					}
					break loop
				}
			}
			updateProgress <- &UpdateProgress{
				Status: Updating,
				Log:    fmt.Sprintf("Query run done: %s, chain id: %d", queryRunID, chainID),
			}

			queryRunResultsResponseRows, err := a.queryRunResults(queryRunID)
			if err != nil {
				updateProgress <- &UpdateProgress{
					Status: Failed,
					Log:    fmt.Sprintf("Failed to get query run result: %s, query run id: %s, chain id: %d", err, queryRunID, chainID),
				}
				return
			}
			updateProgress <- &UpdateProgress{
				Status: Updating,
				Log:    fmt.Sprintf("Query run results got: %d, query run id: %s, chain id: %d", len(queryRunResultsResponseRows), queryRunID, chainID),
			}

			lastAddressRecords, err := a.AddressRecords(chainID)
			if err != nil {
				updateProgress <- &UpdateProgress{
					Status: Failed,
					Log:    fmt.Sprintf("Failed to get let address records: %s, chain id: %d", err, chainID),
				}
				return
			}

			var addressRecords []*AddressRecord
			for _, row := range queryRunResultsResponseRows {
				addressRecord := &AddressRecord{
					ChainID:      chainID,
					Address:      common.HexToAddress(row.Address),
					Name:         row.AddressName,
					LabelType:    row.LabelType,
					LabelSubtype: row.LabelSubtype,
				}
				if len(row.Label) > 0 {
					addressRecord.Label = row.Label
				} else if len(row.ProjectName) > 0 {
					addressRecord.Label = row.ProjectName
				}

				addressRecords = append(addressRecords, addressRecord)
			}
			if err = a.UpsertAddressRecords(addressRecords); err != nil {
				updateProgress <- &UpdateProgress{
					Status: Failed,
					Log:    fmt.Sprintf("Failed to upsert address records: %s, chain id: %d", err, chainID),
				}
				return
			}

			if err = a.UpdateMaintenanceTime(chainID); err != nil {
				updateProgress <- &UpdateProgress{
					Status: Failed,
					Log:    fmt.Sprintf("Failed to update maintenance: %s, chain id: %d", err, chainID),
				}
				return
			}

			updateProgress <- &UpdateProgress{
				Status: Successful,
				Log:    fmt.Sprintf("Address records are up to date, chain id: %d, records: %d, updates: %d", chainID, len(addressRecords)+int(lastAddressRecords), len(addressRecords)-int(lastAddressRecords)),
			}
		}
	}()

	return updateProgress
}
func (a *AddressInf0DB) UpdateSync(logger *Logger) {
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(1)

	updateProgress := a.UpdateAsync()

	go func() {
		for updateProgress_ := range updateProgress {
			if logger != nil {
				switch updateProgress_.Status {
				case Updating:
					logger.Info(updateProgress_.Log)

				case Successful:
					logger.Success(updateProgress_.Log)

				case Warning:
					logger.Warning(updateProgress_.Log)

				case Failed:
					logger.Error(updateProgress_.Log)
				}
			}
		}

		waitGroup.Done()
	}()

	waitGroup.Wait()
}

type queryRunRequest struct {
	ResultTTLHours int    `json:"resultTTLHours"`
	MaxAgeMinutes  int    `json:"maxAgeMinutes"`
	Sql            string `json:"sql"`
	DataSource     string `json:"dataSource"`
	DataProvider   string `json:"dataProvider"`
}
type queryRunResponse struct {
	QueryRequest struct {
		ID             string `json:"id"`
		SqlStatementID string `json:"sqlStatementId"`
		UserID         string `json:"userId"`
		QueryRunID     string `json:"queryRunId"`
	} `json:"queryRequest"`
}

func (a *AddressInf0DB) createQueryRun(query string) (queryRunID string, err error) {
	var (
		createQueryRunRequest_ = &queryRunRequest{
			ResultTTLHours: 1,
			MaxAgeMinutes:  0,
			Sql:            query,
			DataSource:     "snowflake-default",
			DataProvider:   "flipside",
		}
		createQueryRunResponse_ *queryRunResponse
	)
	for retry := 0; retry < retryTimes; retry++ {
		if err = a.flipsideClient.Call(&createQueryRunResponse_, "createQueryRun", createQueryRunRequest_); err != nil {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	if err != nil {
		return "", err
	}

	queryRunID = createQueryRunResponse_.QueryRequest.QueryRunID
	if queryRunID == "" {
		return "", errors.New("query run id is empty")
	}

	return queryRunID, nil
}

type queryRunStatusRequest struct {
	QueryRunId string `json:"queryRunId"`
}
type queryRunStatusResponse struct {
	QueryRun struct {
		State string `json:"state"`
	} `json:"queryRun"`
}

func (a *AddressInf0DB) queryRunStatus(queryRunID string) (err error) {
	var (
		getQueryRunRequest_ = &queryRunStatusRequest{
			QueryRunId: queryRunID,
		}
		getQueryRunResponse_ *queryRunStatusResponse
	)
	for retry := 0; retry < retryTimes; retry++ {
		err = a.flipsideClient.Call(&getQueryRunResponse_, "getQueryRun", getQueryRunRequest_)
		if err != nil && getQueryRunResponse_.QueryRun.State != "QUERY_STATE_SUCCESS" {
			err = errors.New("query run id is empty")
		}

		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		break
	}

	return nil
}

type queryRunResultsRequest struct {
	QueryRunId string `json:"queryRunId"`
	Format     string `json:"format"`
	Page       struct {
		Number int `json:"number"`
		Size   int `json:"size"`
	} `json:"page"`
}
type queryRunResultsResponse struct {
	Rows []*queryRunResultsResponseRow `json:"rows"`
	Page struct {
		CurrentPageNumber int `json:"currentPageNumber"`
		CurrentPageSize   int `json:"currentPageSize"`
		TotalRows         int `json:"totalRows"`
		TotalPages        int `json:"totalPages"`
	} `json:"page"`
}
type queryRunResultsResponseRow struct {
	Address      string `json:"address"`
	AddressName  string `json:"address_name"`
	LabelType    string `json:"label_type"`
	LabelSubtype string `json:"label_subtype"`
	Label        string `json:"label,omitempty"`
	ProjectName  string `json:"project_name,omitempty"`
}

func (a *AddressInf0DB) queryRunResults(queryRunID string) (queryRunResultsResponseRows []*queryRunResultsResponseRow, err error) {
	for page := 1; ; page++ {
		var (
			getQueryRunResultsResponse_ = &queryRunResultsResponse{}
			err                         error
		)
		for retry := 0; retry < retryTimes; retry++ {
			getQueryRunResultsResponse_, err = a.queryRunResultsExactPage(queryRunID, page)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			break
		}
		if err != nil {
			return nil, err
		}

		queryRunResultsResponseRows = append(queryRunResultsResponseRows, getQueryRunResultsResponse_.Rows...)
		if page == getQueryRunResultsResponse_.Page.TotalPages {
			break
		}
	}

	return queryRunResultsResponseRows, nil
}
func (a *AddressInf0DB) queryRunResultsExactPage(queryRunID string, page int) (getQueryRunResultsResponse *queryRunResultsResponse, err error) {
	getQueryRunResultsRequest_ := &queryRunResultsRequest{
		QueryRunId: queryRunID,
		Format:     "json",
		Page: struct {
			Number int `json:"number"`
			Size   int `json:"size"`
		}{
			Number: page,
			Size:   100000,
		},
	}
	if err = a.flipsideClient.Call(&getQueryRunResultsResponse, "getQueryRunResults", getQueryRunResultsRequest_); err != nil {
		return nil, err
	}

	return getQueryRunResultsResponse, nil
}

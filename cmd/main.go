package main

import (
	"fmt"
	"github.com/CryptoNyaRu/addressinf0db"
	"log"
	"time"
)

func main() {
	addressInf0DB, err := addressinf0db.Open("./addressInf0.db", "FLIPSIDE_API_KEY")
	if err != nil {
		log.Fatalln(fmt.Sprintf("Failed to open: %s", err))
	}
	journalMode, err := addressInf0DB.JournalMode()
	if err != nil {
		log.Fatalln(fmt.Sprintf("Failed to get journal mode: %s", err))
	}
	log.Println(fmt.Sprintf("Database connection established, journal mode: %s", journalMode))

	chains, err := addressInf0DB.Chains()
	if err != nil {
		log.Fatalln(fmt.Sprintf("Failed to get chains: %s", err))
	}
	log.Println(fmt.Sprintf("Chains: %d\n", len(chains)))

	for _, chainID := range chains {
		maintenanceTime, err := addressInf0DB.MaintenanceTime(chainID)
		if err != nil {
			log.Fatalln(fmt.Sprintf("Failed to get maintenance time: %s, chain id: %d", err, chainID))
		}
		addressRecords, err := addressInf0DB.AddressRecords(chainID)
		if err != nil {
			log.Fatalln(fmt.Sprintf("Failed to get address records: %s, chain id: %d", err, chainID))
		}

		fmt.Println(fmt.Sprintf("[%d]", chainID))
		fmt.Println(fmt.Sprintf("AddressRecords: %d", addressRecords))
		fmt.Println(fmt.Sprintf("Maintenance   : %s\n", time.Unix(maintenanceTime, 0).String()))
	}

	// UpdateSync
	{
		log.Println("UpdateSync will start in 3 seconds...")
		time.Sleep(3 * time.Second)

		addressInf0DB.UpdateSync(&addressinf0db.Logger{
			Info:    log.Println,
			Success: log.Println,
			Warning: log.Println,
			Error:   log.Println,
		})

		log.Println("UpdateSync done")
	}

}

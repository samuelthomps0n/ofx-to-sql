package main

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/samuelthomps0n/ofx"
	"io"
	"os"
	"strings"
)

func check(e error) {
	if e != nil {
		fmt.Print(e)
	}
}

func main() {

	filePath := flag.String("file", "data.ofx", "Path for the OFX file")
	dbUser := flag.String("user", "homestead", "Database User")
	dbPwd := flag.String("pwd", "secret", "Database Password")
	dbIP := flag.String("ip", "127.0.0.1", "Database IP Address")
	dbPort := flag.String("port", "33060", "Database Port Number")
	dbDatabase := flag.String("db", "admin", "Database name")

	flag.Parse()

	// Connect to the DB
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", *dbUser, *dbPwd, *dbIP, *dbPort, *dbDatabase))
	check(err)

	err = db.Ping()
	check(err)

	defer db.Close()

	// Open the OFX file, then parse it
	var data io.Reader
	data, _ = os.Open(*filePath)
	parsed, _ := ofx.Parse(data)

	var sortCode = strings.Join([]string{parsed.AccountNumber[0:2], parsed.AccountNumber[2:4], parsed.AccountNumber[4:6]}, "-")
	var accountNumber = parsed.AccountNumber[6:]

	stmt, err := db.Prepare("INSERT INTO bank_accounts(sort_code, account_number, balance, balance_updated) VALUES(?, ?, ?, ?) ON DUPLICATE KEY UPDATE balance=?, balance_updated=?")
	check(err)
	res, err := stmt.Exec(sortCode, accountNumber, parsed.Balance, parsed.BalanceUpdated, parsed.Balance, parsed.BalanceUpdated)
	check(err)
	lastId, err := res.LastInsertId()
	check(err)

	fmt.Printf("ID = %d\n", lastId)

	var search = strings.Join([]string{"SELECT id FROM bank_accounts WHERE account_number=", accountNumber}, "")
	var id int
	row := db.QueryRow(search)
	switch err := row.Scan(&id); err {
	case sql.ErrNoRows:
		check(err)
	case nil:
		fmt.Println(id)
	default:
		panic(err)
	}

	// Loop over the transactions, adding them to the SQL DB
	for _, elem := range parsed.Transactions {

		value, _ := elem.Amount.Value.Float64()

		rows, err := db.Query("SELECT COUNT(*) as count FROM bank_transactions WHERE transactional_id = ? AND amount = ?", elem.ID, value)
		check(err)
		defer rows.Close()

		if checkCount(rows) != 1 {

			stmt, err := db.Prepare("INSERT INTO bank_transactions(transactional_id, amount, description, bank_account, date) VALUES(?, ?, ?, ?, ?)")
			check(err)

			res, err := stmt.Exec(elem.ID, value, elem.Description, id, elem.PostedDate)
			check(err)

			lastId, err := res.LastInsertId()
			check(err)

			fmt.Printf("ID = %d\n", lastId)
		}
	}
}

func checkCount(rows *sql.Rows) (count int) {
	for rows.Next() {
		err := rows.Scan(&count)
		check(err)
	}
	return count
}

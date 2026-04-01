package main

import (
    "fmt"
    "os"

    "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
)

func main() {
    if len(os.Args) != 2 {
        fmt.Fprintln(os.Stderr, "usage: repairdb <db-path>")
        os.Exit(2)
    }
    result, err := database.RepairSQLiteDatabase(os.Args[1])
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
    fmt.Printf("repaired=%v partial_import=%v warning=%q backup=%q\n", result.Repaired, result.PartialImport, result.RecoverWarning, result.BackupPath)
}

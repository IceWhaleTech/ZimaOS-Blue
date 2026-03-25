package main

import (
  "context"
  "fmt"
  "log"

  "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
)

func main() {
  svc, err := auth.NewAPIKeyService("/Users/orca/.zimaos-blue/data/blue.db")
  if err != nil {
    log.Fatal(err)
  }
  defer svc.Close()
  key, err := svc.CreateKey(context.Background(), &auth.CreateKeyRequest{
    UserID: "pinchbench",
    Name:   "pinchbench-task21",
    Scopes: []string{"*"},
  })
  if err != nil {
    log.Fatal(err)
  }
  fmt.Print(key.Key)
}

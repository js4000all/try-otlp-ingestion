package main

import (
 "context"
 "flag"
 "fmt"
 "os"
 "os/signal"
 "time"

 "github.com/js4000all/try-otlp-ingestion/internal/generator"
)

func parseStart(s string) (time.Time,error) {
 if t,err:=time.Parse(time.RFC3339,s);err==nil {return t,nil}
 if t,err:=time.Parse("2006-01-02",s);err==nil{return t,nil}
 return time.Time{},fmt.Errorf("invalid -start %q: use YYYY-MM-DD (UTC) or RFC3339",s)
}
func run(args []string) error {
 fs:=flag.NewFlagSet("otlpgen",flag.ContinueOnError)
 startArg:=fs.String("start",time.Now().UTC().Format("2006-01-02"),"YYYY-MM-DD (UTC) or RFC3339")
 days:=fs.Int("days",1,"number of calendar days")
 interval:=fs.Duration("interval",time.Minute,"sample interval (e.g. 1m, 10s)")
 seed:=fs.Int64("seed",1,"random seed")
 configDir:=fs.String("config","config","configuration directory")
 if err:=fs.Parse(args);err!=nil{return err}
 if fs.NArg()!=0{return fmt.Errorf("unexpected arguments: %v",fs.Args())}
 start,err:=parseStart(*startArg);if err!=nil{return err}
 if *days<=0||*interval<=0{return fmt.Errorf("-days and -interval must be positive")}
 cfg,err:=generator.Load(*configDir);if err!=nil{return err}
 ctx,stop:=signal.NotifyContext(context.Background(),os.Interrupt);defer stop()
 return generator.New(cfg,*seed,start).Run(ctx,os.Stdout,*days,*interval)
}
func main(){if err:=run(os.Args[1:]);err!=nil{fmt.Fprintln(os.Stderr,"otlpgen:",err);os.Exit(1)}}

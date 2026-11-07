package main

import ("testing";"time")
func TestParseStart(t *testing.T){
 date,err:=parseStart("2026-11-06")
 if err!=nil||!date.Equal(time.Date(2026,11,6,0,0,0,0,time.UTC)){t.Fatalf("date=%v err=%v",date,err)}
 ts,err:=parseStart("2026-11-06T12:30:00+09:00")
 if err!=nil||ts.UTC().Hour()!=3{t.Fatalf("timestamp=%v err=%v",ts,err)}
 if _,err:=parseStart("2026/11/06");err==nil{t.Fatal("invalid date accepted")}
}

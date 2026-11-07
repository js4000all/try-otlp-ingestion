package generator

import (
 "bytes"
 "context"
 "encoding/json"
 "strings"
 "testing"
 "time"
)

func sampleConfig(t *testing.T)Config{
 t.Helper();c,err:=Load("../../config");if err!=nil{t.Fatal(err)};return c
}
func TestConfigAndSeries(t *testing.T){
 c:=sampleConfig(t);start:=time.Date(2026,11,1,0,0,0,0,time.UTC)
 batch:=New(c,1,start).At(start)
 if len(batch.ResourceMetrics)!=7{t.Fatalf("resources=%d, want 7",len(batch.ResourceMetrics))}
 sitePoints,opsPoints:=0,0
 for _,rm:=range batch.ResourceMetrics{for _,sm:=range rm.ScopeMetrics{for _,m:=range sm.Metrics{for _,dp:=range m.DataPoints{
  if !dp.Timestamp.Equal(start){t.Fatal("timestamp")}
  switch rm.Resource.Attributes["monitoring.domain"]{
  case "site_iot":sitePoints++;if dp.Attributes["sensor_id"]==""{t.Fatal("missing sensor_id")}
  case "ops":opsPoints++;if !m.Monotonic||m.Temporality!="cumulative"||dp.StartTimestamp==nil{t.Fatal("invalid ops sum")}
  default:t.Fatal("missing domain")
  }
 }}}}
 if sitePoints!=16||opsPoints!=3{t.Fatalf("points site=%d ops=%d, want 16 and 3",sitePoints,opsPoints)}
}
func TestReproducibility(t *testing.T){
 c:=sampleConfig(t);at:=time.Date(2026,11,1,0,0,0,0,time.UTC)
 a,_:=json.Marshal(New(c,42,at).At(at));b,_:=json.Marshal(New(c,42,at).At(at))
 if !bytes.Equal(a,b){t.Fatal("same seed returned different data")}
}
func TestHistoricalRun(t *testing.T){
 start:=time.Date(2026,11,1,0,0,0,0,time.UTC)
 g:=New(sampleConfig(t),1,start);var out bytes.Buffer
 if err:=g.Run(context.Background(),&out,1,12*time.Hour);err!=nil{t.Fatal(err)}
 if got:=len(strings.Split(strings.TrimSpace(out.String()),"\n"));got!=2{t.Fatalf("lines=%d, want 2",got)}
}
func TestFutureRunCancellation(t *testing.T){
 start:=time.Now().Add(time.Hour);g:=New(sampleConfig(t),1,start)
 ctx,cancel:=context.WithCancel(context.Background());cancel()
 var out bytes.Buffer
 if err:=g.Run(ctx,&out,1,time.Hour);err!=context.Canceled{t.Fatalf("got %v",err)}
 if out.Len()!=0{t.Fatal("future data emitted early")}
}
func TestInvalidConfig(t *testing.T){
 c:=sampleConfig(t);c.Sites[0].Areas[0].Targets[0].Sensors[0].Metric="missing"
 if err:=c.Validate();err==nil{t.Fatal("expected validation failure")}
}

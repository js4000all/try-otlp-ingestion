package generator

import (
 "bytes"
 "fmt"
 "os"
 "path/filepath"

 "gopkg.in/yaml.v3"
)

type Config struct {
 TargetTypes map[string]TargetType `yaml:"target_types"`
 OpsMetrics map[string][]MetricDef `yaml:"ops_metrics"`
 Sites []Site `yaml:"sites"`
 Readers []Reader `yaml:"readers"`
}
type TargetType struct { Metrics []MetricDef `yaml:"metrics"` }
type MetricDef struct {
 Name string `yaml:"name"`
 Unit string `yaml:"unit"`
 Type string `yaml:"type"`
 SignalType string `yaml:"signal_type"`
 Monotonic bool `yaml:"monotonic"`
 Temporality string `yaml:"temporality"`
 Channels []Channel `yaml:"channels"`
 Generator Pattern `yaml:"generator"`
}
type Channel struct { Position string `yaml:"position"`; Generator Pattern `yaml:"generator"` }
type Pattern struct {
 Pattern string `yaml:"pattern"`
 Baseline float64 `yaml:"baseline"`
 Amplitude float64 `yaml:"amplitude"`
 Period float64 `yaml:"period"`
 Noise float64 `yaml:"noise"`
 Step float64 `yaml:"step"`
 Min float64 `yaml:"min"`
 Max float64 `yaml:"max"`
 SpikeProbability float64 `yaml:"spike_probability"`
 SpikeAmplitude float64 `yaml:"spike_amplitude"`
 Initial float64 `yaml:"initial"`
 Interval float64 `yaml:"interval"`
 IncrementProbability float64 `yaml:"increment_probability"`
 Increment float64 `yaml:"increment"`
}
type Site struct { ID string `yaml:"id"`; Areas []Area `yaml:"areas"` }
type Area struct { ID string `yaml:"id"`; Targets []Target `yaml:"targets"` }
type Target struct { ID string `yaml:"id"`; Type string `yaml:"type"`; Sensors []Sensor `yaml:"sensors"` }
type Sensor struct { ID string `yaml:"id"`; Metric string `yaml:"metric"`; Position string `yaml:"position"` }
type Reader struct { ID string `yaml:"id"`; SiteID string `yaml:"site_id"`; HostID string `yaml:"host_id"`; PLCs []PLC `yaml:"plcs"` }
type PLC struct { ID string `yaml:"id"`; Targets []string `yaml:"targets"` }

func Load(dir string) (Config,error) {
 var c Config
 for _,name:=range []string{"target-metric.yaml","site-area-target.yaml","plc-target.yaml"} {
  b,err:=os.ReadFile(filepath.Join(dir,name));if err!=nil{return c,fmt.Errorf("read %s: %w",name,err)}
  dec:=yaml.NewDecoder(bytes.NewReader(b));dec.KnownFields(true)
  if err:=dec.Decode(&c);err!=nil{return c,fmt.Errorf("parse %s: %w",name,err)}
 }
 return c,c.Validate()
}
func (c Config) Validate() error {
 if len(c.Sites)==0||len(c.Readers)==0 {return fmt.Errorf("sites and readers must not be empty")}
 targets:=map[string]map[string]bool{};sensors:=map[string]bool{};assigned:=map[string]bool{}
 for _,site:=range c.Sites {
  if site.ID==""||targets[site.ID]!=nil{return fmt.Errorf("empty or duplicate site: %q",site.ID)}
  targets[site.ID]=map[string]bool{};areas:=map[string]bool{}
  for _,area:=range site.Areas {
   if area.ID==""||areas[area.ID]{return fmt.Errorf("empty or duplicate area in %s: %q",site.ID,area.ID)};areas[area.ID]=true
   for _,target:=range area.Targets {
    key:=site.ID+"/"+target.ID
    if target.ID==""||targets[site.ID][target.ID]{return fmt.Errorf("empty or duplicate target: %s",key)};targets[site.ID][target.ID]=true
    kind,ok:=c.TargetTypes[target.Type];if !ok{return fmt.Errorf("unknown target type: %s",target.Type)}
    for _,sensor:=range target.Sensors {
     skey:=site.ID+"/"+sensor.ID
     if sensor.ID==""||sensors[skey]{return fmt.Errorf("empty or duplicate sensor: %s",skey)};sensors[skey]=true
     found:=false
     for _,m:=range kind.Metrics {if m.Name==sensor.Metric{for _,ch:=range m.Channels{if ch.Position==sensor.Position{found=true}}}}
     if !found{return fmt.Errorf("no metric channel for %s: %s/%s",skey,sensor.Metric,sensor.Position)}
    }
   }
  }
 }
 if len(c.OpsMetrics["plc-reader"])==0{return fmt.Errorf("missing plc-reader ops metrics")}
 for _,reader:=range c.Readers {
  if reader.ID==""||reader.HostID==""||targets[reader.SiteID]==nil{return fmt.Errorf("invalid reader %q site %q or host",reader.ID,reader.SiteID)}
  for _,plc:=range reader.PLCs {
   if plc.ID==""{return fmt.Errorf("empty PLC id")}
   for _,id:=range plc.Targets {
    key:=reader.SiteID+"/"+id
    if !targets[reader.SiteID][id]||assigned[key]{return fmt.Errorf("unknown or multiply assigned PLC target: %s",key)}
    assigned[key]=true
   }
  }
 }
 for site,ts:=range targets{for target:=range ts{if !assigned[site+"/"+target]{return fmt.Errorf("target has no PLC: %s/%s",site,target)}}}
 return nil
}

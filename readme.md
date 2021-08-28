# Prometheus exporter for dot-com monitoring 
This exporter for the [Prometheus monitoring system](https://prometheus.io/)
calls the [xmlreporter api](https://www.dotcom-monitor.com/wiki/knowledge-base/using-the-xml-reporting-service-xrs/) from dotcom-monitoring to provide metrics on device status.

## dotcom-monitor api format

```xml
<DotcomMonitorConfig>
<Site ID="12" Name="device name" State="Up" Status="ACTIVE"/>
<Site ID="35" Name="device name" State="Up" Status="ACTIVE"/>
</DotcomMonitorConfig>
```

## Building this exporter

The exporter can be built using the Go toolchain:

    go build -o dotcom_monitor

## Using this exporter

The following command will start the exporter, causing it to listen on
TCP port 9423:

    ./dotcom_monitor --dotcom.sites='1234,5422,1232' --dotcom.pid=123457

The exporter has two known reasons for failing a scrape:
 - dotcom-monitor did not return a HTTP 200 OK response
 - dotcom-monitor returned an empty device list

After a succesful scrape, the device status can be one of the following:
 - 0: Status is `Down`
 - 1: Status is `Up`
 - 2: Other possible statuses (e.g. `Postponed`)

Example metrics output:

    # HELP dotcom_device_status Wheter the dotcom alert is active or not
    # TYPE dotcom_device_status gauge
    dotcom_device_status{id="231",name="My name",status="ACTIVE"} 1
    dotcom_device_status{id="123",name="Other Name",status="ACTIVE"} 0
    # HELP dotcom_scrape_success Whether scraping dotcom status was successful.
    # TYPE dotcom_scrape_success gauge
    dotcom_scrape_success 1


## Available flags

    -dotcom.pid string
            This is your account Global Unique Identifier (Configure > Integrations > the Unique Identifier (UID) column). (default "2AA43CD13DDS2CHJ20FGHY85DF203E33")
    
    -dotcom.sites string
            Comma separated list of sites to monitor. Format: 1234,5422,1232
    
    -web.listen-address string
            Address to listen on for web interface and telemetry. (default ":9423")
    
    -web.telemetry-path string
            Path under which to expose metrics. (default "/metrics")

### Using this exporter in Kubernetes

#### Setting up the exporter in Kubernetes

First make sure your `dotcom_monitor` is available in your registry.
```
# Build the docker container with your registry in the tag.
docker build . -t <YOUR-REGISTRY>dotcom-exporter:0.0.1

# Push the docker image so that it is available for Kubernetes. 
docker push <YOUR-REGISTRY>dotcom-exporter:0.0.1
```

We took the liberty to provide you a Kubernetes `deployment.yaml` manifest. You need to replace the following placeholders:
```
# Format: 12345,5354,1231
<CONFIGURE WHICH SITES YOU WANT HERE>

# This is your account Global Unique Identifier (Configure > Integrations > the Unique Identifier (UID) column).
<CONFIGURE YOUR PID HERE>

# You should use the tag that you used in the previous step.
<CONFIGURE YOUR CONTAINER IMAGE HERE> 
```

# orthanctool

Command line utility to interact with the [Orthanc](http://www.orthanc-server.com) DICOM server.

## Usage

```
$ orthanctool help
Usage: orthanctool <flags> <subcommand> <subcommand args>

Subcommands:
        clone            create a complete copy of all instances in an orthanc installation
        recent-patients  yield patient details for most recently changed patients
```

### Clone

```
$ orthanctool clone --orthanc http://A.example/ --dest http://B.example/
```

This copies all instances from A to B. It also watches A for changes and copies new instances as soon as they are added.


### Recent Patients

```
recent-patients --orthanc orthanc_url [command...]:
        Iterates over all patients stored in Orthanc roughly in most recently changed order.
        Outputs JSON with patient ID and LastUpdate timestamp.
        If <command> is given, it will be run for each patient and JSON will be passed to it via stdin.
  -orthanc value
        Orthanc URL
  -poll
        continuously poll for changes (default true)
  -poll-interval int
        poll interval in seconds (default 60)
```

When a handler is given in the command line arguments, `recent-patients` executes that command for 
each patient and passes JSON via stdin.  This passes every patient through `jq`, for example:

```
$ orthanctool recent-patients --orthanc http://A.example/ jq -c '.ID' | head -n 1
"ba5e828d-8d3a73da-40eead54-a5b26022-38f56659"
```

When `--poll` is true (default) then `recent-patients` will watch for changes and yield new 
patients as soon as they are stable.

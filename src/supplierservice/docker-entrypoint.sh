#!/bin/bash
>&2 echo "running the goddamn script"
cron -f 
>&2 echo "cron up"
>&2 echo "running userlookup"
splunk-py-trace python user_lookup.py --port=5003 

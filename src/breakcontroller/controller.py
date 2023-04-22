from flask import Flask, jsonify, request
import sys
import os
import time
import requests
import json

controller = Flask(__name__)
controller.config['JSONIFY_PRETTYPRINT_REGULAR'] = True

ingest_token = os.environ.get('INGEST_TOKEN')
event_url = "https://ingest.us0.signalfx.com/v2/event"
event_headers = {
    "Content-Type": "application/json",
    "X-SF-Token": ingest_token
    }

reviewsproducer_url = 'http://reviewsproducer:8080/config'
reviewsconsumer_url = 'http://reviewsconsumer-2.reviewsconsumer.default.svc.cluster.local:8080/config'

@controller.route('/break')
def break_it():
    r_reviewsproducer = requests.get(reviewsproducer_url + '?api_version=1')
    r_reviewsconsumer = requests.get(reviewsconsumer_url + '?api_version=1')

    t_reviewsproducer = json.loads(r_reviewsproducer.text)
    t_reviewsconsumer = json.loads(r_reviewsconsumer.text)

    deploy_event = [{
            "category": "USER",
            "eventType": "Deployment",
            "dimensions": {
            "environment": "production",
            "service": "userlookup",
            "cluster": "frothly-eks",
            "deployed_by": "Jenna Eagle"
            },
            "properties": {
            "version": "latest",
            "EE-12": "UmVhbGx5PyBBIGJhc2U2NCBlbmNvZGVkIHN0cmluZz8gSG93IGNsZXZlci4uLgo=",
            "sf_environment": "frothly-eks"
            }
        }]

    requests.post(event_url, headers=event_headers,json=deploy_event)

    response_data = {}

    response_data['ReviewsProducerResponse'] = { 
        "StatusCode": r_reviewsproducer.status_code,
        "Response": t_reviewsproducer
    }
    response_data['ReviewsConsumerResponse'] = { 
        "StatusCode": r_reviewsconsumer.status_code,
        "Response": t_reviewsconsumer 
    }
    if int(t_reviewsproducer['ApiVersion']) < 2:
        response_data['EpisodeStatus'] = {"ReviewSentiment": "Negative"}
    else:
        response_data['EpisodeStatus'] = {"ReviewSentiment": "Positive"}

    if int(t_reviewsproducer['ApiVersion']) < 2:
        response_data['EpisodeStatus'] |= {"UserLookupStatus": "Broken"}
    else:
        response_data['EpisodeStatus'] |= {"UserLookupStatus": "Normal"}

    return jsonify(response_data)

@controller.route('/fix')
def fix_it():
    r_reviewsproducer = requests.get(reviewsproducer_url + '?api_version=2')
    r_reviewsconsumer = requests.get(reviewsconsumer_url + '?api_version=2')
    
    t_reviewsproducer = json.loads(r_reviewsproducer.text)
    t_reviewsconsumer = json.loads(r_reviewsconsumer.text)

    rollback_event = [{
            "category": "USER",
            "eventType": "Rollback",
            "dimensions": {
                "environment": "production",
                "service": "userlookup",
                "cluster": "frothly-eks",
                "deployed_by": "Jenna Eagle's Boss"
                },
            "properties": {
                "version": "2",
                "EE-12": "UmVhbGx5PyBBIGJhc2U2NCBlbmNvZGVkIHN0cmluZz8gSG93IGNsZXZlci4uLgo=",
                "sf_environment": "frothly-eks"
                }
            }]

    requests.post(event_url, headers=event_headers,json=rollback_event)

    response_data = {}

    response_data['ReviewsProducerResponse'] = { 
        "StatusCode": r_reviewsproducer.status_code,
        "Response": t_reviewsproducer
    }
    response_data['ReviewsConsumerResponse'] = { 
        "StatusCode": r_reviewsconsumer.status_code,
        "Response": t_reviewsconsumer 
    }
    if int(t_reviewsproducer['ApiVersion']) < 2:
        response_data['EpisodeStatus'] = {"ReviewSentiment": "Negative"}
    else:
        response_data['EpisodeStatus'] = {"ReviewSentiment": "Positive"}

    if int(t_reviewsproducer['ApiVersion']) < 2:
        response_data['EpisodeStatus'] |= {"UserLookupStatus": "Broken"}
    else:
        response_data['EpisodeStatus'] |= {"UserLookupStatus": "Normal"}

    return jsonify(response_data)

@controller.route('/')
def status():
    r_reviewsproducer = requests.get(reviewsproducer_url)
    r_reviewsconsumer = requests.get(reviewsconsumer_url)

    t_reviewsproducer = json.loads(r_reviewsproducer.text)
    t_reviewsconsumer = json.loads(r_reviewsconsumer.text)

    response_data = {}

    response_data['ReviewsProducerResponse'] = { 
        "StatusCode": r_reviewsproducer.status_code,
        "Response": t_reviewsproducer
    }
    response_data['ReviewsConsumerResponse'] = { 
        "StatusCode": r_reviewsconsumer.status_code,
        "Response": t_reviewsconsumer 
    }
    if int(t_reviewsproducer['ApiVersion']) < 2:
        response_data['EpisodeStatus'] = {"ReviewSentiment": "Negative"}
    else:
        response_data['EpisodeStatus'] = {"ReviewSentiment": "Positive"}

    if int(t_reviewsproducer['ApiVersion']) < 2:
        response_data['EpisodeStatus'] |= {"UserLookupStatus": "Broken"}
    else:
        response_data['EpisodeStatus'] |= {"UserLookupStatus": "Normal"}

    return jsonify(response_data)

if __name__ == "__main__":
    controller.run(host='0.0.0.0', port=5000, debug=False)
from flask import Flask, jsonify, request
import json
import random
import time
import logging 

from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import (
    BatchSpanProcessor,
    ConsoleSpanExporter,
)

provider = TracerProvider()
processor = BatchSpanProcessor(ConsoleSpanExporter())
provider.add_span_processor(processor)
trace.set_tracer_provider(provider)

tracer = trace.get_tracer(__name__)

review = Flask(__name__)
positive_reviews = 'positive_reviews.json'
negative_reviews = 'negative_reviews.json'

@review.route('/')
def default():
    return jsonify(message='Success')

@review.route('/get_review')
def get_review():
    review_probability = random.random()
    api_version = request.args.get('api_version')
    customizedSpan = trace.get_current_span()
    customizedSpan.set_attribute("api.version", api_version)

    if int(api_version) >= 2:
        review_chance = 0.90
    else:
        review_chance = 0.10

    if random.random() < review_chance:
        file = open(positive_reviews)
    else:
        file = open(negative_reviews)

    content = file.readlines()
    num_reviews = len(content)
    random_review_int = random.randint(0,num_reviews - 1)
    review_json = json.loads(content[random_review_int])
    customizedSpan.set_attribute("rating", review_json["stars"])
    return content[random_review_int]

if __name__ == "__main__":
    review.run(host ='0.0.0.0', port = 5000, debug = False)
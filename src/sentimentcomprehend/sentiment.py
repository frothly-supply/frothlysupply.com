from flask import Flask, jsonify, request
import boto3
import json

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

sentiment_http = Flask(__name__)
comprehend = boto3.client(service_name='comprehend', region_name='us-east-2')

@sentiment_http.route('/', methods=['POST'])
def get_sentiment():
    with tracer.start_as_current_span("get_sentiment") as span:
        if request.method == 'POST':
            data = request.get_json()
            sentiment = json.dumps(comprehend.detect_sentiment(Text=data["text"], LanguageCode='en'), sort_keys=True, indent=4)
            current_span = trace.get_current_span()
            current_span.set_attribute("span.kind", "SERVER")
            current_span.set_attribute("stars", data["stars"])
            current_span.set_attribute("review", data["text"])
            sentiment_tag = json.loads(sentiment)
            current_span.set_attribute("sentiment", sentiment_tag["Sentiment"])
            return sentiment

if __name__ == "__main__":
    sentiment_http.run(host='0.0.0.0', port=8081, debug=False)
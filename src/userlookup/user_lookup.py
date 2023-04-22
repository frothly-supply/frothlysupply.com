from flask import Flask, jsonify, request
import time
import random

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

user_lookup_app = Flask(__name__)

user_file = 'users.json'

@user_lookup_app.route('/')
def root():
    return jsonify(message='user-lookup-api: please access /user_lookup')

@user_lookup_app.route('/user_lookup')
def user_lookup():
    #api_version = request.args.get('api_version')
    #customizedSpan = trace.get_current_span()
    #customizedSpan.set_attribute("api.version", api_version)
    check_user(2)
    user_id = request.args.get('user_id')
    search_user = '{"user_id":"' + user_id
    searchfile = open(user_file, "r")
    for line in searchfile:
        if search_user in line:
            return line
    searchfile.close()
    return jsonify(message="user not found"), 404

@user_lookup_app.route('/find_user')
def find_user():
    #api_version = request.args.get('api_version')
    #customizedSpan = trace.get_current_span()
    #customizedSpan.set_attribute("api.version", api_version)
    check_user(1)
    user_id = request.args.get('user_id')
    search_user = '{"user_id":"' + user_id
    searchfile = open(user_file, "r")
    for line in searchfile:
        if search_user in line:
            return line
    searchfile.close()
    return jsonify(message="user not found"), 404

def check_user(api_version):
    ctx = trace.get_current_span().get_span_context()
    link_from_current = trace.Link(ctx)

    with tracer.start_as_current_span("check_user", links=[link_from_current]) as new_span:
        if int(api_version) < 2:
            timeout = time.time() + float(random.uniform(1.5, 2.2))
            while True:
                if time.time() > timeout:
                    break

if __name__ == "__main__":
    user_lookup_app.run(host ='0.0.0.0', port = 5003, debug = False)

from flask import Flask, jsonify, request
import time
import random
import csv
import json
import os
from datetime import datetime

from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import (
    BatchSpanProcessor,
    ConsoleSpanExporter,
)
from slack_sdk import WebClient
from slack_sdk.errors import SlackApiError

provider = TracerProvider()
processor = BatchSpanProcessor(ConsoleSpanExporter())
provider.add_span_processor(processor)
trace.set_tracer_provider(provider)

tracer = trace.get_tracer(__name__)

supplier_lookup_app = Flask(__name__)

supplier_file = 'suppliers.csv'

@supplier_lookup_app.route('/')
def root():
    return jsonify(message='supplier-lookup-api: please access /supplier_lookup')

@supplier_lookup_app.route('/supplier_lookup')
def supplier_lookup():
    #api_version = request.args.get('api_version')
    #customizedSpan = trace.get_current_span()
    #customizedSpan.set_attribute("api.version", api_version)
    check_supplier(2)
    supplier_id = request.args.get('supplier_id')
    search_supplier = supplier_id
    searchfile = open(supplier_file, "r")
    for line in searchfile:
        if search_supplier in line:
            return line
    searchfile.close()
    return jsonify(message="supplier not found"), 404

@supplier_lookup_app.route('/find_supplier')
def find_supplier():
    #api_version = request.args.get('api_version')
    #customizedSpan = trace.get_current_span()
    #customizedSpan.set_attribute("api.version", api_version)
    check_supplier(1)
    supplier_id = request.args.get('supplier_id')
    search_supplier = '{"supplier_id":"' + supplier_id
    searchfile = open(supplier_file, "r")
    for line in searchfile:
        if search_supplier in line:
            return line
    searchfile.close()
    return jsonify(message="supplier not found"), 404

@supplier_lookup_app.route('/process_payments')
def process_payments():
    supplier_id = request.args.get('supplier_id')
    payment_amount = request.args.get('amount')
    with open(supplier_file, "r") as csvfile:
        searchfile = csv.reader(csvfile, delimiter=",", quotechar='"', quoting=csv.QUOTE_MINIMAL)
        for row in searchfile:
            if supplier_id == row[0]:
                supplier_name = row[1]
                supplier_routing = row[2]
                supplier_account = row[3]
                os.system('python supplier_payments_notifier.py '+supplier_id+' '+payment_amount+' "'+supplier_name+'" 384520093 1123940587')
                return jsonify(message="Payment processed for supplier: "+supplier_id+" of amount: $"+payment_amount), 200
    

    return jsonify(message="could not find supplier"), 200

def check_supplier(api_version):
    ctx = trace.get_current_span().get_span_context()
    link_from_current = trace.Link(ctx)

    with tracer.start_as_current_span("check_supplier", links=[link_from_current]) as new_span:
        if int(api_version) < 2:
            timeout = time.time() + float(random.uniform(1.5, 2.2))
            while True:
                if time.time() > timeout:
                    break

if __name__ == "__main__":
    supplier_lookup_app.run(host ='0.0.0.0', port = 5004, debug = False)

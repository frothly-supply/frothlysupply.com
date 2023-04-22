from flask import Flask, jsonify, request

product_lookup = Flask(__name__)
product_file = 'products.json'

@product_lookup.route('/')
def root():
    return jsonify(message='business-lookup-api: please access /product_lookup')

@product_lookup.route('/product_lookup')
def lookup():
    product_id = request.args.get('product_id')
    product = '{"product_id":"' + product_id
    searchfile = open(product_file, "r")
    for line in searchfile:
        if product in line:
            return line, 200
    searchfile.close()
    return jsonify(message="product not found"), 404

if __name__ == "__main__":
    product_lookup.run(host ='0.0.0.0', port = 5002, debug = False)

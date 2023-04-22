FROM python:3.10-slim

WORKDIR /app
COPY requirements.txt /app
RUN pip install -r requirements.txt
RUN splk-py-trace-bootstrap

COPY ./product_lookup.py ./products.json /app/

EXPOSE 5002
ENTRYPOINT [ "splunk-py-trace" ]
CMD [ "python", "product_lookup.py" ]
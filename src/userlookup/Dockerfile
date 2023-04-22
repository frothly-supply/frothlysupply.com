FROM python:3.10-slim

WORKDIR /app
COPY requirements.txt /app
RUN pip install -r requirements.txt
RUN pip install splunk-opentelemetry[all]
RUN splk-py-trace-bootstrap

COPY ./user_lookup.py ./users.json /app/

EXPOSE 5003
ENTRYPOINT [ "splunk-py-trace" , "python", "user_lookup.py" , "--port=5003"]
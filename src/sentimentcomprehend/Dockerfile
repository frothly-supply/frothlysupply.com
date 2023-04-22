FROM python:3.10-slim

WORKDIR /app
COPY requirements.txt /app
RUN pip install -r requirements.txt
RUN splk-py-trace-bootstrap

COPY ./sentiment.py /app

EXPOSE 8081
ENTRYPOINT [ "splunk-py-trace" ]
CMD [ "python", "sentiment.py" ]
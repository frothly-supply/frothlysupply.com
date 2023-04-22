FROM python:3.10-slim

WORKDIR /app
COPY requirements.txt /app
COPY ./supplier_payments.csv /app/supplier_payments.csv

RUN pip install -r requirements.txt
RUN pip install splunk-opentelemetry[all]
RUN splk-py-trace-bootstrap
# RUN service cron start
# RUN /etc/init.d/cron start
COPY ./supplier_data.py ./suppliers.csv ./supplier_payments_notifier.py /app/
EXPOSE 5004
ENTRYPOINT [ "splunk-py-trace" , "python", "supplier_data.py" , "--port=5004"]
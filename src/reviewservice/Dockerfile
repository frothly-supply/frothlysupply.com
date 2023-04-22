FROM python:3.10-slim

WORKDIR /app
COPY requirements.txt /app
RUN pip install -r requirements.txt
RUN splk-py-trace-bootstrap

COPY ./review.py ./negative_reviews.json ./positive_reviews.json /app/

EXPOSE 5000
ENTRYPOINT [ "splunk-py-trace" ]
CMD [ "python", "review.py" ]
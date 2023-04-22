FROM locustio/locust

WORKDIR /app
COPY ./loadgen.sh ./locustfile.py /app/

USER root
RUN chmod +x ./loadgen.sh
RUN apt-get update && apt-get install -y \
  bash \
  curl \
  && rm -rf /var/lib/apt/lists/*

USER locust
ENTRYPOINT ./loadgen.sh

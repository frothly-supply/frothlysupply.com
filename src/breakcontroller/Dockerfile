FROM python:3.10-slim

WORKDIR /app
COPY requirements.txt /app
RUN pip install -r requirements.txt

COPY ./controller.py /app

ENTRYPOINT ["python"]
CMD ["controller.py"]


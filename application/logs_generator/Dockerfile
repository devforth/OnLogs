FROM python:3.10-slim-buster
WORKDIR /code
ADD logprint.py .
ADD slow_logs.py .

CMD ["python3", "slow_logs.py"]

# docker build -t logprinter .
# docker run --rm -it --name <any name to identify> logprinter

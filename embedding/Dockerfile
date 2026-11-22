FROM python:3.11-slim

WORKDIR /app
RUN pip install --no-cache-dir sentence-transformers==3.4.1 torch==2.5.1 --index-url https://download.pytorch.org/whl/cpu

COPY service.py requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

ENV MODEL_NAME=sentence-transformers/all-MiniLM-L6-v2
ENV INPUT_DIR=/data/verified
ENV OUTPUT_DIR=/data/embeddings
ENV BATCH_SIZE=32

CMD ["python", "service.py"]
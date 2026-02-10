FROM python:3.9-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy all python files
COPY splitter.py mapper.py reducer.py ./

# Default to splitter, but can override with CMD
EXPOSE 5000
CMD ["python", "splitter.py"]
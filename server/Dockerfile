FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first to leverage Docker layer cache
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy the rest of the application
COPY . .

# Create persistent directories
RUN mkdir -p /app/data /app/exfiltrated

# Expose the Flask API port
EXPOSE 5000

# Run the C2 server
CMD ["python", "app.py"]

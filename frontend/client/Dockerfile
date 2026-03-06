FROM python:3.12-slim
WORKDIR /app
COPY frontend/client/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY frontend/client/ .
EXPOSE 8501
ENTRYPOINT ["streamlit", "run", "app.py", "--server.port=8501", "--server.address=0.0.0.0"]

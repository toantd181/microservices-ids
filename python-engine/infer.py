import os
import logging
import numpy as np
from contextlib import asynccontextmanager
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field, field_validator
from sklearn.ensemble import IsolationForest

logging.basicConfig(
    level=logging.INFO,
    format = "%(acstime)s [%(levelname)s] %(name)s - %(message)s"
)
logger = logging.getLogger("ml-engine")

class NetworkFeatures(BaseModel):
    features: list[float] = Field(
        ...,
        min_length = 6, 
        max_length = 6,
        description = "Exactly 6 numerical network flow features"
    )

    @field_validator("features")
    @classmethod
    def check_no_nan_inf(cls, v):
        for val in v:
            if not np.isfinite(val):
                raise ValueError("Feature values must be finite numbers (no NaN or Inf)")
        return v
    
class PredictionResult(BaseModel):
    prediction: str
    anomaly_score: float
    model_version: str

ml_model: IsolationForest | None = None
MODEL_VERSION = os.getenv("MODEL_VERSION", "1.0.0-dev")

def train_placeholder_model() -> IsolationForest:
    logger.info("Training placehoder IsolationForest model...")
    rng = np.random.default_rng(seed = 42)

    normal_traffic = rng.normal(loc = 0.5, scale = 0.1, size = (200, 6))
    model = IsolationForest(
        n_estimators=100,
        contamination=0.05,
        random_state=42
    )
    model.fit(normal_traffic)
    logger.info("Model trained successfully.")
    return model 

@asynccontextmanager
async def lifespan(app: FastAPI):
    global ml_model
    logger.info(f"ML Engine starting up | model_version = {MODEL_VERSION}")
    ml_model = train_placeholder_model()
    logger.info("ML Engine is ready to serve inference requests")

    yield

    logger.info("ML Engine shutting down gracefully")
    ml_model = None

app = FastAPI(
    title = "IDS ML Engine",
    description = "Internal inference service for the IDS",
    version = MODEL_VERSION,
    lifespan = lifespan,
    docs_url = "/docs" if os.getenv("ENVIRONMENT", "development") == "development" else None,
    redoc_url = None, 
    redirect_slashes = False,
)

@app.get("/health", tags = ["ops"])
async def health_check():
    if ml_model is None:
        raise HTTPException(status_code = 503, detail = "Model not loaded")
    return {"status": "healthy", "model_version": MODEL_VERSION}

@app.post("/predict", response_model = PredictionResult, tags = ["inference"])
async def predict(payload: NetworkFeatures):
    if ml_model is None:
        raise HTTPException(status_code = 503, detail = "Model not loaded")
    try:
        feature_array = np.array(payload.features).reshape(1, -1)
        
        raw_prediction = ml_model.predict(feature_array)[0]
        anomaly_score = float(ml_model.score_samples(feature_array)[0])

        label = "normal" if raw_prediction == 1 else "anomaly"

        logger.info(
            f"Inference complete | prediction = {label} | score = {anomaly_score: .4f}"
        )
        return PredictionResult(
            prediction = label, 
            anomaly_score = anomaly_score,
            model_version = MODEL_VERSION
        )
    except Exception as e:
        logger.error(f"Inference failed: {e}")
        raise HTTPException(status_code = 500, detail = "Inference error")

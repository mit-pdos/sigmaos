#!/usr/bin/env python3
"""
Run once to generate the model artifact:
  python generate_model.py

Writes sklearn_rf/model.joblib relative to this script.
"""

import os
from pathlib import Path

import joblib
import numpy as np
from sklearn.datasets import make_classification
from sklearn.ensemble import RandomForestClassifier

RANDOM_STATE = 42
N_ESTIMATORS = 100
N_FEATURES = 20
N_INFORMATIVE = 15
N_SAMPLES = 10_000

X, y = make_classification(
    n_samples=N_SAMPLES,
    n_features=N_FEATURES,
    n_informative=N_INFORMATIVE,
    random_state=RANDOM_STATE,
)

model = RandomForestClassifier(
    n_estimators=N_ESTIMATORS,
    random_state=RANDOM_STATE,
    n_jobs=-1,
)
model.fit(X, y)

out = Path(__file__).parent / "model.joblib"
joblib.dump(model, out)
print(f"Saved model to {out} ({out.stat().st_size / 1e6:.1f} MB)")

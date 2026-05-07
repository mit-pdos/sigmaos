#!/usr/bin/env python3
"""
SigmaOS image-recognition proc (Python) — junction (handler) variant.

request_json keys: img_bucket, img_key, model_bucket, model_key, kid, async_fetch

async_fetch: "1" to fetch image and model concurrently, "0" for sequential.
If GetRunCoSandbox is true, fetches via delegated S3 RPCs (rpc_idx 0 for
both image and model); otherwise fetches directly by bucket/key.
"""

import io
import os
import time
import numpy as np
from PIL import Image
import onnxruntime as ort
import sigmaos

MEAN = np.array([0.485, 0.456, 0.406], dtype=np.float32)
STD  = np.array([0.229, 0.224, 0.225], dtype=np.float32)


def preprocess(img_bytes: bytes) -> np.ndarray:
    img = Image.open(io.BytesIO(img_bytes)).convert("RGB").resize((224, 224))
    arr = np.array(img, dtype=np.float32) / 255.0   # HWC, [0,1]
    arr = (arr - MEAN) / STD                          # ImageNet normalise
    arr = arr.transpose(2, 0, 1)                      # HWC -> CHW
    return arr[np.newaxis, ...]                        # -> NCHW


def function_handler(request_json):
    is_warmup = request_json["is_warmup"] == "true"
    if is_warmup:
      return "ok"

    for k, v in request_json.get("env", {}).items():
        os.environ[k] = v

    tcp_host = request_json.get("spproxy_tcp_host")
    tcp_port = request_json.get("spproxy_tcp_port")
    if tcp_host and tcp_port:
        clnt = sigmaos.SigmaosClnt(tcp_host=tcp_host, tcp_port=int(tcp_port))
    else:
        clnt = sigmaos.SigmaosClnt()
    clnt.started()

    img_bucket   = request_json["img_bucket"]
    img_key      = request_json["img_key"]
    model_bucket = request_json["model_bucket"]
    model_key    = request_json["model_key"]
    use_async    = request_json.get("async_fetch", "0") == "1"

    transfer_start = time.perf_counter()
    if clnt.get_run_co_sandbox():
        # Zero-copy path: memoryviews backed by shmem, valid for proc lifetime.
        # PIL/BytesIO accept memoryview directly; ORT requires bytes (one copy).
        if use_async:
            fut_img   = clnt.s3_delegated_get_object_view(1, async_=True)
            fut_model = clnt.s3_delegated_get_object_view(0, async_=True)
            img_bytes   = fut_img.result()
            model_bytes = bytes(fut_model.result())
        else:
            img_bytes   = clnt.s3_delegated_get_object_view(1)
            model_bytes = bytes(clnt.s3_delegated_get_object_view(0))
        clnt.log_spawn_latency("Paper.Initialization.TransferState",
                               int((time.perf_counter() - transfer_start) * 1_000_000))
    else:
        if use_async:
            fut_img   = clnt.s3_get_object(img_bucket, img_key,    async_=True)
            fut_model = clnt.s3_get_object(model_bucket, model_key, async_=True)
            img_bytes   = fut_img.result()
            model_bytes = fut_model.result()
        else:
            img_bytes   = clnt.s3_get_object(img_bucket, img_key)
            model_bytes = clnt.s3_get_object(model_bucket, model_key)
        clnt.log_spawn_latency("Paper.Initialization.DownloadState",
                               int((time.perf_counter() - transfer_start) * 1_000_000))
    load_state_start = time.perf_counter()
    sess = ort.InferenceSession(model_bytes)
    clnt.log_spawn_latency("Paper.Initialization.AppLoadState",
                           int((time.perf_counter() - load_state_start) * 1_000_000))

    tensor = preprocess(img_bytes)

    input_name  = sess.get_inputs()[0].name
    output_name = sess.get_outputs()[0].name
    scores = sess.run([output_name], {input_name: tensor})[0][0]

    class_idx = int(np.argmax(scores))
    score     = float(scores[class_idx])

    clnt.exited(sigmaos.STATUS_OK, f"{class_idx},{score}")
    return f"{class_idx},{score}"

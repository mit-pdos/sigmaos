# Copyright 2020-2025 ETH Zurich and the SeBS authors. All rights reserved.
import io
import json
import os
import uuid

import sigmaos


class storage:
    instance = None

    def __init__(self, clnt, use_delegation=False):
        self._clnt = clnt
        if clnt.get_use_shmem():
            clnt.set_use_shmem(True)
        self._delegated = use_delegation
        self._delegated_map = {}  # (bucket, key) -> rpc_idx
        if use_delegation:
            raw = os.environ.get("SEBS_DELEGATED_MAP", "[]")
            for bucket, key, idx in json.loads(raw):
                self._delegated_map[(bucket, key)] = idx

    @staticmethod
    def unique_name(name):
        name, extension = os.path.splitext(name)
        return '{name}.{random}{extension}'.format(
            name=name,
            extension=extension,
            random=str(uuid.uuid4()).split('-')[0]
        )

    @staticmethod
    def get_instance():
        assert storage.instance is not None, "storage singleton not initialized before import"
        return storage.instance

    def download(self, bucket, key, filepath):
        if self._delegated and (bucket, key) in self._delegated_map:
            idx = self._delegated_map[(bucket, key)]
            data = bytes(self._clnt.s3_delegated_get_object_view(idx))
        else:
            data = self._clnt.s3_get_object(bucket, key)
        with open(filepath, 'wb') as f:
            f.write(data)

    def download_stream(self, bucket, key):
        if self._delegated and (bucket, key) in self._delegated_map:
            idx = self._delegated_map[(bucket, key)]
            return bytes(self._clnt.s3_delegated_get_object_view(idx))
        return self._clnt.s3_get_object(bucket, key)

    def upload(self, bucket, key, filepath):
        key_name = storage.unique_name(key)
        with open(filepath, 'rb') as f:
            data = f.read()
        self._clnt.s3_put_object(bucket, key_name, data)
        return key_name

    def upload_stream(self, bucket, key, stream):
        key_name = storage.unique_name(key)
        if hasattr(stream, 'read'):
            data = stream.read()
        else:
            data = bytes(stream)
        self._clnt.s3_put_object(bucket, key_name, data)
        return key_name

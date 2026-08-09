package com.github.anthonycmartin.biceptest;

import com.fasterxml.jackson.databind.JsonNode;
import java.io.IOException;

interface RpcCaller extends AutoCloseable {
    JsonNode call(String method, JsonNode params) throws IOException;

    @Override
    void close();
}
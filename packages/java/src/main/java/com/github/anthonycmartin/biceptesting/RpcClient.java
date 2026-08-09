package com.github.anthonycmartin.biceptesting;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.io.BufferedInputStream;
import java.io.BufferedOutputStream;
import java.io.ByteArrayOutputStream;
import java.io.EOFException;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.concurrent.TimeUnit;

final class RpcClient implements RpcCaller {
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private final Process process;
    private final BufferedInputStream input;
    private final BufferedOutputStream output;
    private long nextId;

    RpcClient(Path executable) throws IOException {
        process = new ProcessBuilder(executable.toString(), "jsonrpc", "--stdio")
                .redirectError(ProcessBuilder.Redirect.DISCARD)
                .start();
        input = new BufferedInputStream(process.getInputStream());
        output = new BufferedOutputStream(process.getOutputStream());
    }

    public synchronized JsonNode call(String method, JsonNode params) throws IOException {
        long requestId = ++nextId;
        ObjectNode request = MAPPER.createObjectNode();
        request.put("jsonrpc", "2.0");
        request.put("id", requestId);
        request.put("method", method);
        request.set("params", params);
        byte[] body = MAPPER.writeValueAsBytes(request);
        output.write(("Content-Length: " + body.length + "\r\n\r\n").getBytes(StandardCharsets.US_ASCII));
        output.write(body);
        output.flush();

        while (true) {
            JsonNode response = readMessage();
            if (!response.has("id") || response.get("id").asLong() != requestId) {
                continue;
            }
            if (response.hasNonNull("error")) {
                JsonNode error = response.get("error");
                throw new IOException("Bicep RPC error " + error.get("code") + ": " + error.get("message").asText());
            }
            return response.get("result");
        }
    }

    private JsonNode readMessage() throws IOException {
        int contentLength = -1;
        while (true) {
            String line = readLine();
            if (line.isEmpty()) {
                break;
            }
            int separator = line.indexOf(':');
            if (separator > 0 && line.substring(0, separator).trim().equalsIgnoreCase("Content-Length")) {
                contentLength = Integer.parseInt(line.substring(separator + 1).trim());
            }
        }
        if (contentLength < 0) {
            throw new IOException("Bicep RPC response did not include Content-Length");
        }
        return MAPPER.readTree(input.readNBytes(contentLength));
    }

    private String readLine() throws IOException {
        ByteArrayOutputStream buffer = new ByteArrayOutputStream();
        while (true) {
            int value = input.read();
            if (value < 0) {
                throw new EOFException("Bicep JSON-RPC process closed its output");
            }
            if (value == '\n') {
                byte[] bytes = buffer.toByteArray();
                int length = bytes.length > 0 && bytes[bytes.length - 1] == '\r' ? bytes.length - 1 : bytes.length;
                return new String(bytes, 0, length, StandardCharsets.US_ASCII);
            }
            buffer.write(value);
        }
    }

    @Override
    public void close() {
        process.destroy();
        try {
            if (!process.waitFor(5, TimeUnit.SECONDS)) {
                process.destroyForcibly();
            }
        } catch (InterruptedException exception) {
            Thread.currentThread().interrupt();
            process.destroyForcibly();
        }
    }
}
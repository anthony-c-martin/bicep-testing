package com.github.anthonycmartin.biceptesting;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.util.Locale;

final class BicepInstaller {
    private BicepInstaller() {}

    static Path install(String version) throws IOException {
        Path directory = Path.of(System.getProperty("user.home"), ".bicep", "bin", "v" + version);
        Files.createDirectories(directory);
        boolean windows = System.getProperty("os.name").toLowerCase(Locale.ROOT).contains("win");
        Path executable = directory.resolve(windows ? "bicep.exe" : "bicep");
        if (Files.exists(executable)) {
            return executable;
        }

        Path temporary = Files.createTempFile(directory, "bicep-download-", ".tmp");
        try {
            HttpRequest request = HttpRequest.newBuilder(URI.create(
                    "https://downloads.bicep.azure.com/v" + version + "/" + artifactName())).build();
            HttpResponse<Path> response;
            try {
                response = HttpClient.newHttpClient().send(request, HttpResponse.BodyHandlers.ofFile(temporary));
            } catch (InterruptedException exception) {
                Thread.currentThread().interrupt();
                throw new IOException("Bicep CLI download was interrupted", exception);
            }
            if (response.statusCode() < 200 || response.statusCode() >= 300) {
                throw new IOException("Bicep CLI download returned HTTP " + response.statusCode());
            }
            temporary.toFile().setExecutable(true, false);
            Files.move(temporary, executable, StandardCopyOption.REPLACE_EXISTING);
            return executable;
        } finally {
            Files.deleteIfExists(temporary);
        }
    }

    private static String artifactName() {
        String os = System.getProperty("os.name").toLowerCase(Locale.ROOT);
        String architecture = System.getProperty("os.arch").toLowerCase(Locale.ROOT);
        String architectureName;
        if (architecture.equals("amd64") || architecture.equals("x86_64")) {
            architectureName = "x64";
        } else if (architecture.equals("aarch64") || architecture.equals("arm64")) {
            architectureName = "arm64";
        } else {
            throw new IllegalStateException("Bicep CLI is not available for " + os + "/" + architecture);
        }
        if (os.contains("win")) {
            return "bicep-win-" + architectureName + ".exe";
        }
        if (os.contains("mac")) {
            return "bicep-osx-" + architectureName;
        }
        if (os.contains("linux")) {
            return "bicep-linux-" + architectureName;
        }
        throw new IllegalStateException("Bicep CLI is not available for " + os + "/" + architecture);
    }
}
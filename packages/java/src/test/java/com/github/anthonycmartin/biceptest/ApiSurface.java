package com.github.anthonycmartin.biceptest;

import java.io.IOException;
import java.lang.reflect.Constructor;
import java.lang.reflect.Method;
import java.lang.reflect.Modifier;
import java.lang.reflect.RecordComponent;
import java.lang.reflect.Type;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Comparator;
import java.util.List;
import java.util.stream.Collectors;

public final class ApiSurface {
    private static final List<Class<?>> PUBLIC_TYPES = List.of(
            BicepTestSession.class,
            DeployOptions.class,
            DeployResult.class,
            DeploymentResource.class,
            SnapshotMetadata.class,
            SnapshotMetadata.Builder.class,
            SnapshotResource.class,
            SnapshotResult.class);

    private ApiSurface() {}

    public static void main(String[] args) throws IOException {
        if (args.length != 1 || !(args[0].equals("--update") || args[0].equals("--check"))) {
            throw new IllegalArgumentException("Expected --update or --check");
        }

        Path baseline = Path.of("..", "..", "api", "java", "bicep-test.txt").normalize();
        String generated = generate();
        if (args[0].equals("--update")) {
            Files.createDirectories(baseline.getParent());
            Files.writeString(baseline, generated, StandardCharsets.UTF_8);
            System.out.println("Updated api/java/bicep-test.txt");
        } else if (!Files.exists(baseline)
                || !Files.readString(baseline, StandardCharsets.UTF_8).replace("\r\n", "\n").equals(generated)) {
            throw new IllegalStateException(
                    "Java public API has changed. Review it and run ApiSurface with --update.");
        } else {
            System.out.println("Java public API is up to date.");
        }
    }

    private static String generate() {
        StringBuilder result = new StringBuilder();
        for (Class<?> type : PUBLIC_TYPES) {
            result.append(type.isRecord() ? "RECORD " : "CLASS ").append(displayName(type)).append('\n');

            List<String> members = new ArrayList<>();
            for (Constructor<?> constructor : type.getDeclaredConstructors()) {
                if (Modifier.isPublic(constructor.getModifiers())) {
                    members.add("constructor " + displayName(type)
                            + parameters(constructor.getGenericParameterTypes()));
                }
            }
            for (Method method : type.getDeclaredMethods()) {
                if (Modifier.isPublic(method.getModifiers()) && !method.isSynthetic()) {
                    String prefix = Modifier.isStatic(method.getModifiers()) ? "static " : "";
                    members.add(prefix + displayName(method.getGenericReturnType()) + " " + method.getName()
                            + parameters(method.getGenericParameterTypes()) + exceptions(method.getExceptionTypes()));
                }
            }
            if (type.isRecord()) {
                for (RecordComponent component : type.getRecordComponents()) {
                    members.add("component " + displayName(component.getGenericType()) + " " + component.getName());
                }
            }
            members.stream().sorted().forEach(member -> result.append("  ").append(member).append('\n'));
            result.append('\n');
        }
        return result.toString();
    }

    private static String parameters(Type[] types) {
        return Arrays.stream(types).map(ApiSurface::displayName).collect(Collectors.joining(", ", "(", ")"));
    }

    private static String exceptions(Class<?>[] types) {
        if (types.length == 0) {
            return "";
        }
        return Arrays.stream(types)
                .map(ApiSurface::displayName)
                .sorted(Comparator.naturalOrder())
                .collect(Collectors.joining(", ", " throws ", ""));
    }

    private static String displayName(Type type) {
        if (type instanceof Class<?> classType && classType.isArray()) {
            return displayName(classType.getComponentType()) + "[]";
        }
        return type.getTypeName().replace('$', '.');
    }
}
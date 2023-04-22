FROM maven:3.8.5-openjdk-11-slim AS build  
COPY src /usr/src/app/src  
COPY pom.xml /usr/src/app  
RUN mvn -f /usr/src/app/pom.xml clean package

FROM openjdk:11
COPY --from=build /usr/src/app/target/consumer-1.2.jar /consumer-1.2.jar 
RUN wget -q https://github.com/signalfx/splunk-otel-java/releases/latest/download/splunk-otel-javaagent.jar
CMD ["java", "-javaagent:./splunk-otel-javaagent.jar", "-Dsplunk.profiler.enabled=true", "-Dsplunk.profiler.period.threaddump=1000", "-Dsplunk.metrics.enabled=true", "-jar", "-Dotel.instrumentation.common.experimental.suppress-messaging-receive-spans=true", "consumer-1.2.jar"]

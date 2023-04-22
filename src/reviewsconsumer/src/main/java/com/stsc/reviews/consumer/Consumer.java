package com.stsc.reviews.consumer;

import com.mongodb.client.MongoClient;
import com.mongodb.client.MongoClients;
import com.mongodb.client.MongoCollection;
import com.stsc.reviews.consumer.controllers.ConfigController;

import org.apache.http.conn.ssl.NoopHostnameVerifier;

import org.apache.http.impl.client.CloseableHttpClient;
import org.apache.http.impl.client.HttpClients;
import org.apache.kafka.clients.consumer.ConsumerRecord;
import org.apache.kafka.clients.consumer.ConsumerRecords;
import org.apache.kafka.clients.consumer.KafkaConsumer;
import org.apache.kafka.common.serialization.StringDeserializer;
import org.bson.Document;
import org.json.*;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.HttpEntity;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.http.client.HttpComponentsClientHttpRequestFactory;

import org.springframework.web.client.RestTemplate;

import java.net.URI;
import java.security.KeyManagementException;
import java.security.NoSuchAlgorithmException;
import java.security.cert.X509Certificate;
import java.time.Duration;
import java.util.*;

import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManager;
import javax.net.ssl.X509TrustManager;


public class Consumer {
    private static Logger logger = LoggerFactory.getLogger(Consumer.class.getName());

    public static JSONObject mergeJSONObjects(JSONObject json1, JSONObject json2) {
        JSONObject mergedJSON = new JSONObject();
        try {
            mergedJSON = new JSONObject(json1, JSONObject.getNames(json1));
            for (String crunchifyKey : JSONObject.getNames(json2)) {
                mergedJSON.put(crunchifyKey, json2.get(crunchifyKey));
            }
        } catch (JSONException e) {
            throw new RuntimeException("JSON Exception" + e);
        }
        return mergedJSON;
    }

    public static void RunConsumer() throws KeyManagementException, NoSuchAlgorithmException {

        TrustManager[] trustAllCerts = new TrustManager[] {
            new X509TrustManager() {
                public java.security.cert.X509Certificate[] getAcceptedIssuers() {
                    return new X509Certificate[0];
                }
                public void checkClientTrusted(
                        java.security.cert.X509Certificate[] certs, String authType) {
                }
                public void checkServerTrusted(
                        java.security.cert.X509Certificate[] certs, String authType) {
                }
            }
        };  

        SSLContext sslContext = SSLContext.getInstance("SSL");
        sslContext.init(null, trustAllCerts, new java.security.SecureRandom()); 
        CloseableHttpClient httpClient = HttpClients.custom()
                .setSSLContext(sslContext)
                .setSSLHostnameVerifier(NoopHostnameVerifier.INSTANCE)
                .build();   
        HttpComponentsClientHttpRequestFactory customRequestFactory = new HttpComponentsClientHttpRequestFactory();
        customRequestFactory.setHttpClient(httpClient);

        RestTemplate restTemplate = new RestTemplate( customRequestFactory );
    
        //Establish connection to MongoDB
        String connectionString = "mongodb://mongodb/O11y";
		String bootstrapServers = "kafka-0.kafka-headless.default.svc.cluster.local:9092";
		String groupId = "reviews_to_db";
		String topic = "reviews";
		Properties properties = new Properties();
		properties.setProperty("bootstrap.servers", bootstrapServers);
		properties.setProperty("key.deserializer", StringDeserializer.class.getName());
		properties.setProperty("value.deserializer", StringDeserializer.class.getName());
		properties.setProperty("group.id", groupId);
		properties.setProperty("auto.offset.reset", "earliest");
		properties.setProperty("enable.auto.commit", "false");
		properties.setProperty("max.poll.records", "1");
	
        try (MongoClient mongoClient = MongoClients.create(connectionString)) {
            MongoCollection O11yCollection = mongoClient.getDatabase("O11y").getCollection("O11yCollection");
            // Set up Kafka Connection
            KafkaConsumer<String, String> consumer = new KafkaConsumer(properties);
            consumer.subscribe(Arrays.asList(topic));

            // Consume continuously from Reviews topic
            while (true) {
                ConsumerRecords<String, String> records = consumer.poll(Duration.ofMillis(100L));
                Iterator var8 = records.iterator();
                ConsumerRecord record;
                var8 = records.iterator();
                while (var8.hasNext()) {
                    record = (ConsumerRecord) var8.next();

                    logger.info("Partition: " + record.partition() + ", Offset: " + record.offset() + ", Record Count: " + records.count());
                    //extract business id
                    JSONObject reviewJson = new JSONObject(record.value().toString());

                    //extract user and business id
                    String productId = reviewJson.getString("product_id");
                    String userId = reviewJson.getString("user_id");
                    
                    String userEndpoint;
                    if (ConfigController.getApiVersion() < 2) {
                        userEndpoint = "find_user";
                    } else {
                        userEndpoint = "user_lookup";
                    }

                    URI productUri = URI.create("http://productlookup:5002/product_lookup?product_id=" + productId);
                    URI userUri = URI.create("http://userlookup:5003/" + userEndpoint + "?user_id=" + userId);
                    URI comprehendUri = URI.create("http://sentiment-comprehend:8081");
                    URI splunkHecUri = URI.create("https://splunk.frothlysupply.com:8088/services/collector");
                    
                    HttpHeaders reviewHeaders = new HttpHeaders();
                    reviewHeaders.setContentType(MediaType.APPLICATION_JSON);
                    HttpEntity<String> reviewEntity = new HttpEntity<>(reviewJson.toString(), reviewHeaders);

                    try {

                        ResponseEntity<String> sentiment = restTemplate.postForEntity(comprehendUri, reviewEntity, String.class);
                        ResponseEntity<String> productResult = restTemplate.getForEntity(productUri, String.class);
                        ResponseEntity<String> userResult = restTemplate.getForEntity(userUri, String.class);
    
                        //Create a combined json
                        JSONObject reviewProduct = mergeJSONObjects(reviewJson, new JSONObject(productResult.getBody()));
                        JSONObject reviewProductUser = mergeJSONObjects(reviewProduct, new JSONObject(userResult.getBody()));
                        JSONObject combinedReview = mergeJSONObjects(reviewProductUser, new JSONObject(sentiment.getBody()));
                        
                        //Create HEC json Object
                        JSONObject splunkHecEvent = new JSONObject();
                        splunkHecEvent.put("time", new Date().getTime());
                        splunkHecEvent.put("source", "reviewsconsumer");
                        splunkHecEvent.put("sourcetype", "product_review");
                        splunkHecEvent.put("event", combinedReview);

                        HttpHeaders splunkHecHeaders = new HttpHeaders(){{set("Authorization", "Splunk d7d0a57a-431f-49fe-80af-c7312ec9f352");}};
                        HttpEntity<String> splunkHecEntity = new HttpEntity<>(splunkHecEvent.toString(), splunkHecHeaders);
                        ResponseEntity<String> splunkHecResponse = restTemplate.postForEntity(splunkHecUri, splunkHecEntity, String.class);
                        logger.info(splunkHecResponse.getBody());

                        //Insert into MongoDB
                        Document myDoc = Document.parse(combinedReview.toString());
                        logger.info("Review Event:\n" + myDoc.toJson());
                        O11yCollection.insertOne(myDoc);
                        consumer.commitAsync();

                    } catch (Exception e){
                        logger.error("HTTP Request Failed with exception:\n" + e.toString());
                    }


                }
            }
        }
    }
}

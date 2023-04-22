package com.stsc.reviews.producer.controllers;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;


@RestController
public class ProducerController {
    final Logger logger = LoggerFactory.getLogger(ProducerController.class);

    @Value(value = "${reviewservice.url}")
    private String reviewServiceUrl;

    @Autowired
    private KafkaTemplate<String, String> producer;

    @GetMapping("/reviews")
    public void GetReview() throws IOException, InterruptedException{
        String reviewServiceUri = reviewServiceUrl + "?api_version=" + ConfigController.getApiVersion();
        for (int i=0; i<ConfigController.getNumReviews(); i++ ) {      
            HttpResponse<String> review = HttpClient.newHttpClient()
                .send(HttpRequest.newBuilder()
                .uri(URI.create(reviewServiceUri))
                .build(), 
                HttpResponse.BodyHandlers.ofString());

            if (review.statusCode() == 200){
                producer.send("reviews",review.body());
            } else {
                logger.error(review.body());
            }
            
        }
        producer.flush(); 
    }
}

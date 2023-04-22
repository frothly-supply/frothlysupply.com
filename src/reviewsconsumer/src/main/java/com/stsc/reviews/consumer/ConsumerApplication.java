package com.stsc.reviews.consumer;

import java.security.KeyManagementException;
import java.security.NoSuchAlgorithmException;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class ConsumerApplication {

    public static void main(String[] args) throws KeyManagementException, NoSuchAlgorithmException {
        SpringApplication.run(ConsumerApplication.class, args);
        Consumer.RunConsumer();     
    }
}

package com.stsc.reviews.consumer.controllers;

import org.springframework.web.bind.annotation.RestController;

import com.stsc.reviews.consumer.Config;

import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.ModelAttribute;


@RestController
public class ConfigController { 

    private static Config reviewsConfig = new Config(){{setApiVersion(2);}};

    @GetMapping("/config")
    public String setConfig(@ModelAttribute() Config config, 
                            Model model, 
                            @RequestParam(value="api_version", required=false) Integer api_version) {

        if (api_version != null){
            reviewsConfig.setApiVersion(api_version);
        }
        return reviewsConfig.toString();
	}

    public static Integer getApiVersion(){
        return reviewsConfig.getApiVersion();
    }

}
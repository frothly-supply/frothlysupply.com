package com.stsc.reviews.consumer;

public class Config {
	private Integer api_version;

	public Integer getApiVersion() {
		return api_version;
	}

	public void setApiVersion(Integer api_version) {
		this.api_version = api_version;
	}

	@Override
	public String toString() {
		return "{\"ApiVersion\":\"" + api_version +"\"}";
	}

}

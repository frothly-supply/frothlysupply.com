package com.stsc.reviews.producer;

public class Config {
    private Integer num_reviews;
	private Integer api_version;

	public Integer getNumReviews() {
		return num_reviews;
	}

	public void setNumReviews(Integer num_reviews) {
		this.num_reviews = num_reviews;
	}

	public Integer getApiVersion() {
		return api_version;
	}

	public void setApiVersion(Integer api_version) {
		this.api_version = api_version;
	}

	@Override
	public String toString() {
		return "{\"ReviewsPerSubmission\":\"" + num_reviews + "\",\"ApiVersion\":\"" + api_version +"\"}";
	}

}


PUT _ilm/policy/bsc_financial_policy
{
  "policy": {
    "phases": {
      "hot": {
        "actions": {
          "rollover": {
            "max_age": "7d",
            "max_size": "20gb"
          }
        }
      }
    }
  }
}

PUT _index_template/bsc_financial_template
{
  "index_patterns": [
    "pro_bsc_financial-*"
  ],
  "template": {
    "settings": {
      "index.lifecycle.name": "bsc_financial_policy",
      "index.lifecycle.rollover_alias": "bsc_financial_write",
      "analysis": {
        "normalizer": {
          "lowercase_normalizer": {
            "filter": [
              "lowercase"
            ],
            "type": "custom",
            "char_filter": []
          }
        }
      }
    },
    "mappings": {
      "properties": {
        "amount_hi": {
          "type": "long"
        },
        "amount_lo": {
          "type": "long"
        },
        "amount_raw": {
          "type": "keyword",
          "doc_values": false
        },
        "block_number": {
          "type": "long"
        },
        "block_time": {
          "type": "date"
        },
        "chain_id": {
          "type": "long"
        },
        "create_at": {
          "type": "date"
        },
        "credit_subject": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "credit_subject_code": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "credit_subject_name": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "credit_subject_path": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "credit_subject_root_code": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "debit_subject": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "debit_subject_code": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "debit_subject_name": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "debit_subject_path": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "debit_subject_root_code": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "decimals": {
          "type": "integer"
        },
        "direction": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "from": {
          "type": "keyword",
          "normalizer": "lowercase_normalizer"
        },
        "from_role": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "ledger_type": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "symbol": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "to": {
          "type": "keyword",
          "normalizer": "lowercase_normalizer"
        },
        "to_role": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "token": {
          "type": "keyword",
          "normalizer": "lowercase_normalizer"
        },
        "tx_from": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "tx_hash": {
          "type": "keyword",
          "normalizer": "lowercase_normalizer"
        },
        "tx_to": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "tx_type": {
          "type": "keyword"
        },
        "value_usd": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        }
      }
    }
  }
}


PUT /pro_bsc_financial-000001
{
  "aliases": {
    "bsc_financial_write": {
      "is_write_index": true
    },
    "bsc_financial_all": {}
  }
}
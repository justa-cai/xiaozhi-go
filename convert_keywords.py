#!/usr/bin/env python3

"""
Script to convert Chinese wake words to the proper phonetic token format
using Sherpa-onnx's text2token functionality.
"""

import os
import subprocess
import sys

def convert_keywords_to_tokens(keywords_text, tokens_file, tokens_type="ppinyin"):
    """
    Convert Chinese keywords to phonetic tokens using Sherpa-onnx's text2token.
    
    Args:
        keywords_text: List of keywords in format "keyword[:boost][#threshold][@original]"
        tokens_file: Path to tokens.txt file
        tokens_type: Type of tokens (ppinyin, fpinyin, etc.)
    """
    # Create temporary input file
    with open("temp_keywords_input.txt", "w", encoding="utf-8") as f:
        for keyword in keywords_text:
            f.write(keyword + "\n")
    
    try:
        # Run the text2token script
        cmd = [
            "python3", 
            "third/sherpa-onnx/scripts/text2token.py",
            "--text", "temp_keywords_input.txt",
            "--tokens", tokens_file,
            "--tokens-type", tokens_type,
            "--output", "temp_converted_keywords.txt"
        ]
        
        result = subprocess.run(cmd, capture_output=True, text=True)
        
        if result.returncode != 0:
            print(f"Error running text2token: {result.stderr}")
            return None
        
        # Read the converted keywords
        with open("temp_converted_keywords.txt", "r", encoding="utf-8") as f:
            converted = f.read().strip()
        
        return converted
        
    finally:
        # Clean up temporary files
        for temp_file in ["temp_keywords_input.txt", "temp_converted_keywords.txt"]:
            if os.path.exists(temp_file):
                os.remove(temp_file)

if __name__ == "__main__":
    # Example usage
    print("Converting Chinese keywords to phonetic tokens...")
    
    # Example keywords - you can modify these as needed
    keywords = [
        "你好小智",  # nǐ hǎo xiǎo zhì
        "小智同学"   # xiǎo zhì tóngxué
    ]
    
    tokens_file = "models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/tokens.txt"
    
    if not os.path.exists(tokens_file):
        print(f"Error: {tokens_file} not found!")
        sys.exit(1)
    
    converted = convert_keywords_to_tokens(keywords, tokens_file, "ppinyin")
    
    if converted:
        print("Converted keywords:")
        print(converted)
        print("\nFormat: phonetic_tokens @ original_text")
        print("Use this format when updating the Go code.")
    else:
        print("Conversion failed.")
# Live Captions

Live Captions is a desktop application providing real-time speech-to-text subtitles directly on your screen. Built using Go, Wails, and the Vosk speech recognition engine, it operates entirely offline. The software functions as a transparent, always-on-top overlay suitable for meetings, video playback, and general accessibility.

## Core Features

* **Offline Processing:** The application analyzes audio locally on your machine. It requires no internet connection, ensuring your conversations remain entirely private.

* **Transparent Overlay:** The user interface consists of a frameless, draggable window. It sits seamlessly over other open applications without obstructing your workflow.

* **Multiple Languages:** Users can switch between English, Spanish, French, German, Japanese, Chinese, and several other languages. The software automatically downloads and applies new language models in the background upon selection.

* **Audio File Transcription:** You can process pre-recorded media. By uploading audio files, the built-in FFmpeg engine converts and transcribes the content into a text document within seconds.

* **Automatic Saving:** The program maintains a permanent record of your transcripts. These text files are automatically generated and saved to a dedicated folder in your Documents directory.

* **User Customization:** You can adjust the caption font size on the fly and toggle between light and dark display modes to suit your viewing preferences.

## System Requirements

* **Operating System:** Windows 10 or Windows 11 (64-bit).

* *Note:* Legacy systems such as Windows 7 and Windows 8 are not supported due to modern WebView2 rendering requirements.

## Installation Guide

1. Navigate to the **Releases** tab on the right side of this repository page.

2. Download the latest setup executable.

3. Run the installer. During the setup process, you may check a box to include the FFmpeg Audio Engine if you wish to enable pre-recorded audio transcription.

4. Launch the application from your Windows Start Menu.

### Where the files are stored

The default transcription file is located in C:\Users\USERNAME\Documents\Transcriptions

The downloaded models are located at C:\Users\USERNAME\AppData\Roaming\LiveCaptions

## Usage Instructions

* **Live Captions** When the app is open, the audio played on the device will automatically transcribe into the captions inside the box.

* **Changing the Language:** Open the settings menu by clicking the gear icon. Select a different language from the available dropdown list. The application will fetch the necessary files and switch over automatically.

* **Transcribing a File:** Within the settings menu, click the upload button. Select your desired audio file and wait a few moments for the completion message. Your finished text document will appear in your default save folder.

* **File exports** The circle on the top near the settings button turns on live transcription, when it's red, it means it is active, click again to stop live transcription.
  
* **Languages supported**
  * English
  * Spanish
  * French
  * German
  * Japanese
  * Chinese(Mandarin)
  * Italian
  * Hindi
  * Russian
  * Portuguese.

## Privacy

Because the speech recognition process is completely localized via the Vosk engine, no audio data is ever transmitted over the internet or recorded by external servers. All voice processing remains strictly on your local hardware.

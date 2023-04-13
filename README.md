This project is a telegram bot, which converts voice messages to text. This is backend service which receives telegram messages and passes them to model servers https://github.com/ReshitkoM/msg_recognition-model_server 

Installation
Local install
1) install go 1.19
2) create config.yaml with your bot token in root. Example config is located in Docked/backend/exampleConfig.yaml
3) to run service execute make

Docker
1) clone https://github.com/ReshitkoM/msg_recognition-model_server
2) cd Docker
3) Create config.yaml and config in backend/ and model/ folders respectively
4) make build PATH_TO_MODEL_SERVER=path_to_model_server_dir
5) make run

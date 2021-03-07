import React from "react";
import styles from './DefaultRoom.module.sass'

class DefaultRoom extends React.Component<{}, any> {
    private intervalId: NodeJS.Timeout | undefined;
    
    constructor(props: {}) {
        super(props);
        this.state = {
            message: "メッセージです。",
        };
    }
    
    componentDidMount() {
        this.intervalId = setInterval(() => {
            // todo reload data
            this.setState({
                message: "新しい",
            });
        }, 1000);
    }
    
    componentWillUnmount() {
        if (this.intervalId) {
            clearInterval(this.intervalId);
        }
    }
    
    render() {
        return (
            <div id={styles.defaultRoom}>
                {this.state.message}
            </div>);
    }
}

export default DefaultRoom;

<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:include href="include.xslt"/>
	<xsl:template match="/">
		<item>
			<value>
				<xsl:value-of select="/root/item"/>
			</value>
			<xsl:call-template name="foobar"/>
		</item>
	</xsl:template>

</xsl:stylesheet>